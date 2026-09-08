package wechat

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"likeadmin/backend/internal/cache"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestOAuthByCodeKeepsAccessToken(t *testing.T) {
	orig := httpClient
	t.Cleanup(func() { httpClient = orig })
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "oauth2/access_token") {
			return jsonResp(`{"openid":"oid1","access_token":"tok1","unionid":"u1"}`), nil
		}
		if strings.Contains(r.URL.Path, "sns/userinfo") {
			return jsonResp(`{"nickname":"n","headimgurl":"http://a.png"}`), nil
		}
		return jsonResp(`{}`), nil
	})}
	s, err := OAuthByCode("app", "sec", "code")
	if err != nil {
		t.Fatal(err)
	}
	if s.Openid != "oid1" || s.AccessToken != "tok1" || s.Nickname != "n" || s.Unionid != "u1" {
		t.Fatalf("%+v", s)
	}
}

func TestOAuthByCodeAllowsOpenidWithoutToken(t *testing.T) {
	// PHP WeChatOaService::getOaResByCode only requires openid.
	orig := httpClient
	t.Cleanup(func() { httpClient = orig })
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(`{"openid":"oid2"}`), nil
	})}
	s, err := OAuthByCode("app", "sec", "code")
	if err != nil || s.Openid != "oid2" || s.AccessToken != "" {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestPublishMenuErrorJSONEncodesBody(t *testing.T) {
	orig := httpClient
	t.Cleanup(func() { httpClient = orig })
	appID := "appid-test-menu-err"
	cache.Del("wechat_access_token_" + appID)
	body := `{"errcode":40013,"errmsg":"invalid appid"}`
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "cgi-bin/token") {
			return jsonResp(`{"access_token":"t","expires_in":7200}`), nil
		}
		return jsonResp(body), nil
	})}
	err := PublishMenu(appID, "sec", []any{map[string]any{"name": "首页", "type": "click", "key": "k"}})
	if err == nil {
		t.Fatal("expected error")
	}
	quoted, _ := json.Marshal(body)
	want := "保存发布菜单失败" + string(quoted)
	if err.Error() != want {
		t.Fatalf("got %q want %q", err.Error(), want)
	}
}
