package pcshop

import (
	"net/url"
	"testing"
)

func TestTargetNewsGoesToPCNotMobile(t *testing.T) {
	if got := Target("/pages/news/news", nil); got != "/pc/information" {
		t.Fatalf("news list %q", got)
	}
	q := url.Values{"id": []string{"3"}}
	if got := Target("/pages/news_detail/news_detail", q); got != "/pc/information/detail/3" {
		t.Fatalf("news detail %q", got)
	}
	if got := Target("/pages/collection/collection", nil); got != "/pc/user/collection" {
		t.Fatalf("collection %q", got)
	}
	if got := Target("/packages/pages/user_wallet/user_wallet", nil); got != "/pc/" {
		t.Fatalf("packages %q", got)
	}
	if Target("/platform/login", nil) != "" {
		t.Fatal("unrelated path")
	}
}

func TestInternalDropsPCPrefix(t *testing.T) {
	if got := Internal("/pages/news/news", nil); got != "/information" {
		t.Fatalf("news list %q", got)
	}
	q := url.Values{"id": []string{"6"}}
	if got := Internal("/pages/news_detail/news_detail", q); got != "/information/detail/6" {
		t.Fatalf("news detail %q", got)
	}
	if got := Internal("/pages/index/index", nil); got != "/" {
		t.Fatalf("home %q", got)
	}
}
