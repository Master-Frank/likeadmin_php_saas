package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

type SaveResult struct {
	URI    string
	Engine string
}

var (
	qiniuUploadURL = "https://upload.qiniup.com/"
	qiniuRSURL     = "https://rs.qiniu.com"
)

func Delete(c *gin.Context, uri string) error {
	if uri == "" {
		return nil
	}
	engine := cfgsvc.GetString(c, "storage", "default", "local")
	if engine == "" {
		engine = "local"
	}
	key := ObjectKey(uri)
	if engine == "local" {
		abs := filepath.Join(config.C.App.PublicDir, key)
		if _, err := os.Stat(abs); err != nil {
			return nil
		}
		return os.Remove(abs)
	}
	cfg := asMap(cfgsvc.Get(c, "storage", engine, map[string]any{}))
	switch engine {
	case "qiniu":
		return deleteQiniu(cfg, key)
	case "aliyun":
		return deleteAliyun(cfg, key)
	case "qcloud":
		return deleteQcloud(cfg, key)
	default:
		return nil
	}
}

func Fetch(c *gin.Context, srcURL, rel string) (SaveResult, error) {
	srcURL = strings.TrimSpace(srcURL)
	if srcURL == "" {
		return SaveResult{}, fmt.Errorf("empty url")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(srcURL)
	if err != nil {
		return SaveResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return SaveResult{}, fmt.Errorf("远程文件下载失败: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SaveResult{}, err
	}
	if len(body) == 0 {
		return SaveResult{}, fmt.Errorf("远程文件为空")
	}
	ct := resp.Header.Get("Content-Type")
	return Save(c, rel, bytes.NewReader(body), int64(len(body)), ct)
}

func Save(c *gin.Context, rel string, r io.Reader, size int64, contentType string) (SaveResult, error) {
	engine := cfgsvc.GetString(c, "storage", "default", "local")
	if engine == "" {
		engine = "local"
	}
	if engine == "local" {
		abs := filepath.Join(config.C.App.PublicDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			return SaveResult{}, err
		}
		f, err := os.Create(abs)
		if err != nil {
			return SaveResult{}, err
		}
		defer f.Close()
		if _, err = io.Copy(f, r); err != nil {
			return SaveResult{}, err
		}
		return SaveResult{URI: rel, Engine: "local"}, nil
	}
	cfg := asMap(cfgsvc.Get(c, "storage", engine, map[string]any{}))
	body, err := io.ReadAll(r)
	if err != nil {
		return SaveResult{}, err
	}
	key := strings.TrimLeft(rel, "/")
	switch engine {
	case "qiniu":
		if err := putQiniu(cfg, key, body, contentType); err != nil {
			return SaveResult{}, err
		}
	case "aliyun":
		if err := putAliyun(cfg, key, body, contentType); err != nil {
			return SaveResult{}, err
		}
	case "qcloud":
		if err := putQcloud(cfg, key, body, contentType); err != nil {
			return SaveResult{}, err
		}
	default:
		return SaveResult{}, fmt.Errorf("未知存储引擎")
	}
	return SaveResult{URI: rel, Engine: engine}, nil
}

// ObjectKey turns a stored URI or CDN URL into the bucket/local relative key.
func ObjectKey(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if i := strings.IndexAny(uri, "?#"); i >= 0 {
		uri = uri[:i]
	}
	if strings.Contains(uri, "://") {
		if u, err := url.Parse(uri); err == nil {
			uri = u.Path
		}
	}
	uri = strings.TrimLeft(uri, "/")
	if i := strings.Index(uri, "uploads/"); i > 0 {
		uri = uri[i:]
	}
	return uri
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

func str(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func putQiniu(cfg map[string]any, key string, body []byte, contentType string) error {
	ak, sk, bucket := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("七牛云配置不完整")
	}
	policy, _ := json.Marshal(map[string]any{"scope": bucket + ":" + key, "deadline": time.Now().Unix() + 3600})
	encoded := base64.URLEncoding.EncodeToString(policy)
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(encoded))
	token := ak + ":" + base64.URLEncoding.EncodeToString(mac.Sum(nil)) + ":" + encoded
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("token", token)
	_ = w.WriteField("key", key)
	fw, err := w.CreateFormFile("file", filepath.Base(key))
	if err != nil {
		return err
	}
	if _, err = fw.Write(body); err != nil {
		return err
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, qiniuUploadURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if contentType != "" {
		_ = contentType
	}
	return do(req)
}

func putAliyun(cfg map[string]any, key string, body []byte, contentType string) error {
	ak, sk, bucket := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("阿里云OSS配置不完整")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	scheme, host := aliyunHost(cfg)
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "PUT\n\n" + contentType + "\n" + date + "\n/" + bucket + "/" + key
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(canon))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req, err := http.NewRequest(http.MethodPut, scheme+"://"+host+"/"+key, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "OSS "+ak+":"+sig)
	return do(req)
}

func putQcloud(cfg map[string]any, key string, body []byte, contentType string) error {
	ak, sk, bucket, region := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket"), str(cfg, "region")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("腾讯云COS配置不完整")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	scheme, host := qcloudHost(cfg, region, bucket)
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "put\n/" + key + "\n\nhost=" + host + "\n"
	stringToSign := "sha1\n" + date + "\n" + fmt.Sprintf("%x", sha1.Sum([]byte(canon))) + "\n"
	signKey := hmacSHA1Hex(sk, date)
	sig := hmacSHA1Hex(signKey, stringToSign)
	auth := fmt.Sprintf("q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s;%s&q-key-time=%s;%s&q-header-list=host&q-url-param-list=&q-signature=%s",
		ak, date, date, date, date, sig)
	req, err := http.NewRequest(http.MethodPut, scheme+"://"+host+"/"+key, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", auth)
	return do(req)
}

func deleteQiniu(cfg map[string]any, key string) error {
	ak, sk, bucket := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("七牛云存储配置不完整")
	}
	entry := base64.URLEncoding.EncodeToString([]byte(bucket + ":" + key))
	path := "/delete/" + entry
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(path + "\n"))
	auth := ak + ":" + base64.URLEncoding.EncodeToString(mac.Sum(nil))
	req, err := http.NewRequest(http.MethodPost, qiniuRSURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QBox "+auth)
	return doDelete(req)
}

func deleteAliyun(cfg map[string]any, key string) error {
	ak, sk, bucket := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("阿里云OSS配置不完整")
	}
	scheme, host := aliyunHost(cfg)
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "DELETE\n\n\n" + date + "\n/" + bucket + "/" + key
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(canon))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req, err := http.NewRequest(http.MethodDelete, scheme+"://"+host+"/"+key, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", "OSS "+ak+":"+sig)
	return doDelete(req)
}

func deleteQcloud(cfg map[string]any, key string) error {
	ak, sk, bucket, region := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket"), str(cfg, "region")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("腾讯云COS配置不完整")
	}
	scheme, host := qcloudHost(cfg, region, bucket)
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "delete\n/" + key + "\n\nhost=" + host + "\n"
	stringToSign := "sha1\n" + date + "\n" + fmt.Sprintf("%x", sha1.Sum([]byte(canon))) + "\n"
	signKey := hmacSHA1Hex(sk, date)
	sig := hmacSHA1Hex(signKey, stringToSign)
	auth := fmt.Sprintf("q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s;%s&q-key-time=%s;%s&q-header-list=host&q-url-param-list=&q-signature=%s",
		ak, date, date, date, date, sig)
	req, err := http.NewRequest(http.MethodDelete, scheme+"://"+host+"/"+key, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Host", host)
	req.Header.Set("Authorization", auth)
	return doDelete(req)
}

func aliyunHost(cfg map[string]any) (scheme, host string) {
	domain := firstNonEmpty(str(cfg, "domain"), str(cfg, "endpoint"))
	fallback := str(cfg, "bucket") + ".oss-cn-hangzhou.aliyuncs.com"
	if domain == "" {
		if region := strings.TrimPrefix(str(cfg, "region"), "oss-"); region != "" {
			fallback = str(cfg, "bucket") + ".oss-" + region + ".aliyuncs.com"
		}
	}
	return storageHost(domain, fallback)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func storageHost(domain, fallback string) (scheme, host string) {
	scheme = "https"
	raw := strings.TrimSpace(domain)
	switch {
	case strings.HasPrefix(raw, "https://"):
		host = strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		scheme = "http"
		host = strings.TrimPrefix(raw, "http://")
	default:
		host = raw
	}
	host = strings.TrimRight(host, "/")
	if host == "" {
		host = fallback
	}
	return scheme, host
}

func qcloudHost(cfg map[string]any, region, bucket string) (scheme, host string) {
	if region != "" {
		return "https", bucket + ".cos." + region + ".myqcloud.com"
	}
	return storageHost(str(cfg, "domain"), bucket+".cos.ap-guangzhou.myqcloud.com")
}

func doDelete(req *http.Request) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("存储删除失败: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

func hmacSHA1Hex(key, msg string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(msg))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func do(req *http.Request) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("存储上传失败: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}
