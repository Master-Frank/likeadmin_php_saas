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

func Delete(c *gin.Context, uri string) error {
	if uri == "" {
		return nil
	}
	engine := cfgsvc.GetString(c, "storage", "default", "local")
	if engine == "" {
		engine = "local"
	}
	key := strings.TrimLeft(uri, "/")
	if engine == "local" {
		abs := filepath.Join(config.C.App.PublicDir, key)
		if _, err := os.Stat(abs); err != nil {
			return nil
		}
		return os.Remove(abs)
	}
	cfg := asMap(cfgsvc.Get(c, "storage", engine, map[string]any{}))
	switch engine {
	case "qiniu", "aliyun", "qcloud":
		if str(cfg, "access_key") == "" || str(cfg, "secret_key") == "" || str(cfg, "bucket") == "" {
			return nil
		}
		return nil
	default:
		return nil
	}
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
	req, err := http.NewRequest(http.MethodPost, "https://upload.qiniup.com/", &buf)
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
	ak, sk, bucket, domain := str(cfg, "access_key"), str(cfg, "secret_key"), str(cfg, "bucket"), str(cfg, "domain")
	if ak == "" || sk == "" || bucket == "" {
		return fmt.Errorf("阿里云OSS配置不完整")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	host := strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	if host == "" {
		host = bucket + ".oss-cn-hangzhou.aliyuncs.com"
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "PUT\n\n" + contentType + "\n" + date + "\n/" + bucket + "/" + key
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(canon))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	url := "https://" + host + "/" + key
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
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
	host := bucket + ".cos." + region + ".myqcloud.com"
	if region == "" {
		host = strings.TrimPrefix(strings.TrimPrefix(str(cfg, "domain"), "https://"), "http://")
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := "put\n/" + key + "\n\nhost=" + host + "\n"
	stringToSign := "sha1\n" + date + "\n" + fmt.Sprintf("%x", sha1.Sum([]byte(canon))) + "\n"
	signKey := hmacSHA1Hex(sk, date)
	sig := hmacSHA1Hex(signKey, stringToSign)
	auth := fmt.Sprintf("q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s;%s&q-key-time=%s;%s&q-header-list=host&q-url-param-list=&q-signature=%s",
		ak, date, date, date, date, sig)
	req, err := http.NewRequest(http.MethodPut, "https://"+host+"/"+key, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", auth)
	return do(req)
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
