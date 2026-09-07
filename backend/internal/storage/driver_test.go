package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutAndDeleteQiniuFixture(t *testing.T) {
	var gotKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/" {
			_ = r.ParseMultipartForm(1 << 20)
			gotKey = r.FormValue("key")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/delete/") {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	oldUp, oldRS := qiniuUploadURL, qiniuRSURL
	qiniuUploadURL, qiniuRSURL = srv.URL+"/", srv.URL
	t.Cleanup(func() { qiniuUploadURL, qiniuRSURL = oldUp, oldRS })

	cfg := map[string]any{"access_key": "ak", "secret_key": "sk", "bucket": "bucket"}
	if err := putQiniu(cfg, "uploads/a.txt", []byte("hi"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if gotKey != "uploads/a.txt" {
		t.Fatalf("key=%s", gotKey)
	}
	if err := deleteQiniu(cfg, "uploads/a.txt"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotAuth, "QBox ak:") {
		t.Fatalf("auth=%s", gotAuth)
	}
}

func TestPutAndDeleteAliyunFixture(t *testing.T) {
	var method, path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if r.Method == http.MethodPut && string(body) != "payload" {
			http.Error(w, "bad body", 400)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	cfg := map[string]any{
		"access_key": "ak", "secret_key": "sk", "bucket": "bucket", "domain": srv.URL,
	}
	if err := putAliyun(cfg, "uploads/b.txt", []byte("payload"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/uploads/b.txt" || !strings.HasPrefix(auth, "OSS ak:") {
		t.Fatalf("put %s %s %s", method, path, auth)
	}
	if err := deleteAliyun(cfg, "uploads/b.txt"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/uploads/b.txt" {
		t.Fatalf("delete %s %s", method, path)
	}
}

func TestPutQcloudUsesObjectKey(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if !strings.Contains(r.Header.Get("Authorization"), "q-sign-algorithm=sha1") {
			http.Error(w, "no sign", 403)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	cfg := map[string]any{
		"access_key": "ak", "secret_key": "sk", "bucket": "bucket", "domain": srv.URL,
	}
	if err := putQcloud(cfg, "uploads/c.txt", []byte("x"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if path != "/uploads/c.txt" {
		t.Fatalf("path=%s", path)
	}
}

func TestStorageHostHTTP(t *testing.T) {
	scheme, host := storageHost("http://127.0.0.1:9000/oss", "fallback")
	if scheme != "http" || host != "127.0.0.1:9000/oss" {
		t.Fatalf("%s %s", scheme, host)
	}
	scheme, host = storageHost("", "bucket.example")
	if scheme != "https" || host != "bucket.example" {
		t.Fatalf("fallback %s %s", scheme, host)
	}
}

func TestAliyunHostUsesRegion(t *testing.T) {
	scheme, host := aliyunHost(map[string]any{"bucket": "bkt", "region": "cn-beijing"})
	if scheme != "https" || host != "bkt.oss-cn-beijing.aliyuncs.com" {
		t.Fatalf("%s %s", scheme, host)
	}
	scheme, host = aliyunHost(map[string]any{"bucket": "bkt", "domain": "http://oss.local/path"})
	if scheme != "http" || host != "oss.local/path" {
		t.Fatalf("domain %s %s", scheme, host)
	}
}

func TestUnknownEngineMessage(t *testing.T) {
	err := unknownEngineErr("ftp")
	if err == nil || err.Error() != "未找到存储引擎类: ftp" {
		t.Fatalf("%v", err)
	}
}

func TestDeleteCloudMissingConfig(t *testing.T) {
	if err := deleteQiniu(map[string]any{}, "k"); err == nil {
		t.Fatal("qiniu")
	}
	if err := deleteAliyun(map[string]any{}, "k"); err == nil {
		t.Fatal("aliyun")
	}
	if err := deleteQcloud(map[string]any{}, "k"); err == nil {
		t.Fatal("qcloud")
	}
}
