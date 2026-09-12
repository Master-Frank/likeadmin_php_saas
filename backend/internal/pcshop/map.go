package pcshop

import (
	"net/url"
	"strings"
)

// Target maps decorate/shop links (uniapp-style /pages and /packages paths)
// onto the PC Nuxt site under /pc/. PC banners open these paths in a new tab.
func Target(path string, query url.Values) string {
	path = strings.TrimSuffix(path, "/")
	if path == "" || strings.Contains(path, "..") {
		return ""
	}
	switch path {
	case "/pages/news/news":
		return "/pc/information"
	case "/pages/news_detail/news_detail":
		if id := query.Get("id"); id != "" {
			return "/pc/information/detail/" + url.PathEscape(id)
		}
		return "/pc/information"
	case "/pages/index/index", "/pages/search/search", "/pages":
		return "/pc/"
	case "/pages/collection/collection":
		return "/pc/user/collection"
	case "/pages/user/user", "/pages/user_data/user_data":
		return "/pc/user/info"
	case "/pages/user_set/user_set":
		return "/pc/account/security"
	case "/pages/agreement/agreement":
		if typ := query.Get("type"); typ != "" {
			return "/pc/policy/" + url.PathEscape(typ)
		}
		return "/pc/"
	}
	if strings.HasPrefix(path, "/pages/") || path == "/packages" || strings.HasPrefix(path, "/packages/") {
		return "/pc/"
	}
	return ""
}

// Internal is Target without the /pc prefix so NuxtLink (baseURL /pc/) does
// not produce /pc/pc/information.
func Internal(path string, query url.Values) string {
	target := Target(path, query)
	if target == "" {
		return ""
	}
	if target == "/pc" || target == "/pc/" {
		return "/"
	}
	return strings.TrimPrefix(target, "/pc")
}
