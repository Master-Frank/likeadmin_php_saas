package upgrade

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/util"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// downloadClient matches PHP UpgradeLogic::downFile curl:
// CURLOPT_SSL_VERIFYPEER=false and no FOLLOWLOCATION.
// A 302 HTML interstitial must not be saved as the upgrade zip.
var downloadClient = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func serverRoot() string {
	if pub := config.C.App.PublicDir; pub != "" {
		return filepath.Dir(pub)
	}
	wd, _ := os.Getwd()
	for d := wd; d != "" && d != "/"; d = filepath.Dir(d) {
		if st, err := os.Stat(filepath.Join(d, "server", "app")); err == nil && st.IsDir() {
			return filepath.Join(d, "server")
		}
		if st, err := os.Stat(filepath.Join(d, "app", "common")); err == nil && st.IsDir() {
			return d
		}
	}
	return "server"
}

// backendRoot is the Go module root (sibling of server/). Upgrade zips may
// ship project/backend/ with the same layout as this directory.
func backendRoot() string {
	root := serverRoot()
	candidate := filepath.Join(filepath.Dir(root), "backend")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate
	}
	wd, _ := os.Getwd()
	for d := wd; d != "" && d != "/"; d = filepath.Dir(d) {
		if st, err := os.Stat(filepath.Join(d, "cmd", "api")); err == nil && st.IsDir() {
			return d
		}
		if st, err := os.Stat(filepath.Join(d, "backend", "cmd", "api")); err == nil && st.IsDir() {
			return filepath.Join(d, "backend")
		}
	}
	return candidate
}

func versionListsKey(pageNo, pageSize int) string {
	if pageNo == 0 && pageSize == 0 {
		return "version_lists"
	}
	return fmt.Sprintf("version_lists%d", pageNo)
}

func versionListsURL(pageNo, pageSize int) string {
	if pageNo == 0 || pageSize == 0 {
		return fmt.Sprintf("%s/indexapi/version/lists?type=2&page=1&action=lists&product_code=%s", upgradeBase(), ProductCode)
	}
	return fmt.Sprintf("%s/indexapi/version/lists?type=2&page_no=%d&page_size=%d&page=1&action=lists&product_code=%s",
		upgradeBase(), pageNo, pageSize, ProductCode)
}

func hasVersionLists(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	lists, _ := payload["lists"].([]any)
	return len(lists) > 0
}

// GetRemoteVersion mirrors UpgradeLogic::getRemoteVersion.
// Empty remote payloads are not cached and are retried so a single
// timeout does not stick for 30 minutes or fail a later request.
func GetRemoteVersion(pageNo, pageSize int) map[string]any {
	key := versionListsKey(pageNo, pageSize)
	if raw, ok := cache.Get(key); ok && raw != "" {
		var payload map[string]any
		if json.Unmarshal([]byte(raw), &payload) == nil && hasVersionLists(payload) {
			return payload
		}
	}
	remote := versionListsURL(pageNo, pageSize)
	var last map[string]any
	for attempt := 0; attempt < 3; attempt++ {
		payload := getJSON(remote)
		if hasVersionLists(payload) {
			if b, err := json.Marshal(payload); err == nil {
				cache.Set(key, string(b), 30*time.Minute)
			}
			return payload
		}
		if payload != nil {
			last = payload
		}
		if attempt < 2 {
			time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
		}
	}
	if last != nil {
		return last
	}
	return map[string]any{}
}

// VersionByID mirrors getVersionDataById.
func VersionByID(id any) map[string]any {
	want := util.ToString(id)
	if want == "" || want == "0" {
		return nil
	}
	payload := GetRemoteVersion(0, 0)
	lists, _ := payload["lists"].([]any)
	for _, item := range lists {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		if util.ToString(m["id"]) == want || fmt.Sprint(m["id"]) == want {
			return m
		}
	}
	return nil
}

// HasPermission mirrors PHP !$result['has_permission'].
func HasPermission(result map[string]any) bool {
	if result == nil {
		return false
	}
	switch v := result["has_permission"].(type) {
	case bool:
		return v
	case string:
		return v == "1" || strings.EqualFold(v, "true")
	default:
		return util.ToInt(result["has_permission"]) != 0
	}
}

// Verify mirrors UpgradeLogic::verify.
func Verify(domain string, versionID any, link string) map[string]any {
	u := fmt.Sprintf("%s/indexapi/version/verify?domain=%s&type=2&version_id=%s&link=%s&action=verify&product_code=%s",
		upgradeBase(), url.QueryEscape(domain), url.QueryEscape(util.ToString(versionID)), url.QueryEscape(link), ProductCode)
	data := getVerifyJSON(u)
	if data == nil {
		return map[string]any{"has_permission": false, "link": "", "msg": ""}
	}
	return data
}

// AddLog mirrors UpgradeLogic::addLog.
func AddLog(domain string, versionID any, updateType any, ok bool, errMsg string) {
	ver := VersionByID(versionID)
	if ver == nil {
		ver = map[string]any{"id": versionID, "version_no": ""}
	}
	form := url.Values{}
	form.Set("version_id", util.ToString(ver["id"]))
	form.Set("version_no", util.ToString(ver["version_no"]))
	form.Set("domain", domain)
	form.Set("type", "2")
	form.Set("product_code", ProductCode)
	form.Set("update_type", util.ToString(updateType))
	if ok {
		form.Set("status", "1")
	} else {
		form.Set("status", "0")
	}
	form.Set("error", errMsg)
	if fixtureDir() != "" {
		return
	}
	_, _ = httpClient.Post(upgradeBase()+"/indexapi/version/log", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}

func getJSON(remote string) map[string]any {
	wrap := fetchJSON(remote)
	if wrap == nil {
		return nil
	}
	data, _ := wrap["data"].(map[string]any)
	return data
}

// getVerifyJSON keeps PHP verify's data-or-default shape and also surfaces
// a top-level envelope msg when data.msg is empty (remote license replies
// sometimes put ip未授权 on the envelope, not inside data).
func getVerifyJSON(remote string) map[string]any {
	wrap := fetchJSON(remote)
	if wrap == nil {
		return nil
	}
	return parseVerifyEnvelope(wrap)
}

func parseVerifyEnvelope(wrap map[string]any) map[string]any {
	data, _ := wrap["data"].(map[string]any)
	if data == nil {
		data = map[string]any{"has_permission": false, "link": "", "msg": ""}
	}
	if util.ToString(data["msg"]) == "" {
		if msg := util.ToString(wrap["msg"]); msg != "" {
			data["msg"] = msg
		}
	}
	return data
}

func fetchJSON(remote string) map[string]any {
	if fixtureDir() != "" {
		// Fixture mode never hits the live upgrade API, even when the
		// file is missing or the body is empty / non-JSON.
		return parseJSONBody(readFixture(remote))
	}
	resp, err := httpClient.Get(remote)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return parseJSONBody(body)
}

func parseJSONBody(body []byte) map[string]any {
	if len(body) == 0 {
		return nil
	}
	var wrap map[string]any
	if json.Unmarshal(body, &wrap) != nil {
		return nil
	}
	return wrap
}

func upgradeBase() string {
	if u := strings.TrimSpace(os.Getenv("LIKEADMIN_UPGRADE_BASE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return BaseURL
}

func fixtureDir() string {
	return strings.TrimSpace(os.Getenv("LIKEADMIN_UPGRADE_FIXTURE"))
}

func readFixture(remote string) []byte {
	dir := fixtureDir()
	if dir == "" {
		return nil
	}
	name := "lists.json"
	switch {
	case strings.Contains(remote, "/verify") || strings.Contains(remote, "action=verify"):
		name = "verify.json"
	case strings.Contains(remote, "/log"):
		name = "log.json"
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil
	}
	return b
}

var pkgLink = map[int]string{
	1: "package_link",
	2: "package_link",
	3: "pc_package_link",
	4: "uniapp_package_link",
	5: "web_package_link",
	6: "integral_package_link",
	8: "kefu_package_link",
}

// PkgLinkName maps update_type to the remote link field name.
func PkgLinkName(updateType int) string {
	if name, ok := pkgLink[updateType]; ok {
		return name
	}
	return "未知类型"
}
