package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/util"
)

const ProductCode = "462953db655787cb99deb5893f8d523a"
const BaseURL = "https://server.mddai.cn"

// LocalVersion reads PHP ./upgrade/version.json, creating it from project.version if missing.
func LocalVersion() string {
	dir := filepath.Join(serverRoot(), "upgrade")
	path := filepath.Join(dir, "version.json")
	if b, err := os.ReadFile(path); err == nil {
		var data map[string]any
		if json.Unmarshal(b, &data) == nil {
			if v := util.ToString(data["version"]); v != "" {
				return v
			}
		}
	}
	ver := config.C.Project.Version
	if ver == "" {
		ver = "1.0.0"
	}
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(path, versionJSON(ver), 0644)
	return ver
}

func versionJSON(ver string) []byte {
	b, _ := json.Marshal(map[string]string{"version": ver})
	return b
}

// WriteLocalVersion persists the installed version after a successful upgrade.
func WriteLocalVersion(ver string) error {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return nil
	}
	for _, r := range ver {
		if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '.' && r != '_' && r != '-' {
			return nil
		}
	}
	dir := filepath.Join(serverRoot(), "upgrade")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "version.json"), versionJSON(ver), 0644)
}

// FormatLists mirrors UpgradeLogic::formatLists.
func FormatLists(rows []any, pageNo int, local string) []map[string]any {
	if local == "" {
		local = LocalVersion()
	}
	out := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		ver := util.ToString(m["version_no"])
		m["version_str"] = ""
		m["able_update"] = 0
		if local == ver {
			m["version_str"] = "您的系统当前处于此版本"
		}
		if local < ver {
			m["version_str"] = "系统可更新至此版本"
			m["able_update"] = 1
		}
		m["new_version"] = 0
		notice := []any{}
		if util.ToInt(m["uniapp_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布手机端前端前台")
		}
		if util.ToInt(m["pc_admin_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布前端PC后台")
		}
		if util.ToInt(m["pc_shop_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布前端PC前台")
		}
		notice = append(notice, util.ToString(m["publish_content"]))
		m["notice"] = notice

		add, optimize, repair := []any{}, []any{}, []any{}
		if contents, ok := m["update_content"].([]any); ok {
			for _, raw := range contents {
				cm, _ := raw.(map[string]any)
				if cm == nil {
					continue
				}
				fn := util.ToString(cm["update_function"])
				switch util.ToInt(cm["type"]) {
				case 1:
					add = append(add, "新增:"+fn)
				case 2:
					optimize = append(optimize, "优化:"+fn)
				case 3:
					repair = append(repair, "修复:"+fn)
				}
			}
		}
		contentDesc := append([]any{}, add...)
		contentDesc = append(contentDesc, optimize...)
		contentDesc = append(contentDesc, repair...)
		m["add"] = add
		m["optimize"] = optimize
		m["repair"] = repair
		m["content_desc"] = contentDesc
		delete(m, "update_content")
		out = append(out, m)
	}
	if len(out) > 0 {
		if pageNo == 1 {
			out[0]["new_version"] = 1
		} else {
			out[0]["new_version"] = 0
		}
	}
	return out
}
