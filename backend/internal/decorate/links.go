package decorate

import (
	"encoding/json"
	"net/url"
	"strings"

	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pcshop"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

// ArticleIDTitle is a title-keyed article id used to map template picker rows
// onto tenant copies.
type ArticleIDTitle struct {
	ID    uint
	Title string
}

// MapTemplateArticleIDs maps tenant_id=0 template ids onto tenant copies that
// share the same title. Same-id rows (sharded seed copies that reuse 1/2/3)
// are skipped so decorate JSON is left alone.
func MapTemplateArticleIDs(templates, copies []ArticleIDTitle) map[uint]uint {
	out := map[uint]uint{}
	byTitle := map[string]uint{}
	for _, tpl := range templates {
		if tpl.Title == "" || tpl.ID == 0 {
			continue
		}
		byTitle[tpl.Title] = tpl.ID
	}
	if len(byTitle) == 0 {
		return out
	}
	seen := map[string]bool{}
	for _, a := range copies {
		if a.Title == "" || seen[a.Title] {
			continue
		}
		seen[a.Title] = true
		if old, ok := byTitle[a.Title]; ok && old != a.ID {
			out[old] = a.ID
		}
	}
	return out
}

// TenantArticleIDMap loads template articles from tplDB and tenant copies from
// tenantDB, then maps template ids onto the copies.
func TenantArticleIDMap(tplDB, tenantDB *gorm.DB, tenantID uint) map[uint]uint {
	if tplDB == nil || tenantDB == nil || tenantID == 0 {
		return map[uint]uint{}
	}
	var tpls []model.Article
	tplDB.Where("tenant_id = 0 AND is_show = 1 AND delete_time IS NULL").Find(&tpls)
	if len(tpls) == 0 {
		return map[uint]uint{}
	}
	titles := make([]string, 0, len(tpls))
	templates := make([]ArticleIDTitle, 0, len(tpls))
	for _, tpl := range tpls {
		if tpl.Title == "" {
			continue
		}
		templates = append(templates, ArticleIDTitle{ID: tpl.ID, Title: tpl.Title})
		titles = append(titles, tpl.Title)
	}
	if len(titles) == 0 {
		return map[uint]uint{}
	}
	var copies []model.Article
	tenantDB.Where("tenant_id = ? AND is_show = 1 AND delete_time IS NULL AND title IN ?", tenantID, titles).
		Order("id asc").Find(&copies)
	refs := make([]ArticleIDTitle, 0, len(copies))
	copyByTitle := map[string]uint{}
	for _, a := range copies {
		refs = append(refs, ArticleIDTitle{ID: a.ID, Title: a.Title})
		if _, ok := copyByTitle[a.Title]; !ok {
			copyByTitle[a.Title] = a.ID
		}
	}
	out := MapTemplateArticleIDs(templates, refs)
	// Decorate JSON may store another tenant's copy id (demo seed used 6) or a
	// template id. Map every visible row with the same title onto this tenant.
	var aliases []model.Article
	tplDB.Where("is_show = 1 AND delete_time IS NULL AND title IN ?", titles).Find(&aliases)
	for _, a := range aliases {
		nid, ok := copyByTitle[a.Title]
		if !ok || a.ID == nid {
			continue
		}
		out[a.ID] = nid
	}
	return out
}

// RemapArticleIDs rewrites decorate JSON so template article ids (tenant_id=0
// picker rows) become the tenant copies. Used when cloning decorate pages.
func RemapArticleIDs(raw string, idMap map[uint]uint) string {
	if raw == "" || len(idMap) == 0 {
		return raw
	}
	v, ok := decodeJSONValue(raw)
	if !ok {
		return raw
	}
	walkJSON(v, func(m map[string]any) {
		remapLinkIDs(m, idMap)
	})
	return encodeJSONValue(v)
}

// CopyBannerLinksByImage copies article/shop links from src banner slides onto
// dst slides that use the same image file. PC and mobile decorate pages share
// default banner PNGs but historically stored different link types.
func CopyBannerLinksByImage(dst, src string) string {
	return copyBannerLinksByImage(dst, src, false)
}

// FillDefaultBannerLinksByImage copies src links onto dest slides that still
// have the stock empty/shop placeholders (资讯中心 / 我的收藏 / 空链接).
func FillDefaultBannerLinksByImage(dst, src string) string {
	return copyBannerLinksByImage(dst, src, true)
}

func copyBannerLinksByImage(dst, src string, onlyDefault bool) string {
	links := bannerSlideLinksByImage(src)
	if len(links) == 0 || dst == "" {
		return dst
	}
	v, ok := decodeJSONValue(dst)
	if !ok {
		return dst
	}
	applyBannerSlideLinks(v, links, onlyDefault)
	return encodeJSONValue(v)
}

// RewriteForPC maps uniapp shop/article paths onto Nuxt routes under /pc/
// (without the /pc prefix, because the PC app already uses baseURL /pc/).
func RewriteForPC(raw string, resolve func(uint) uint) string {
	if raw == "" {
		return raw
	}
	v, ok := decodeJSONValue(raw)
	if !ok {
		return raw
	}
	walkJSON(v, func(m map[string]any) {
		rewritePCLink(m, resolve)
	})
	return encodeJSONValue(v)
}

func decodeJSONValue(raw string) (any, bool) {
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil, false
	}
	return v, true
}

func encodeJSONValue(v any) string {
	out := util.EncodeJSON(v)
	if out == "" {
		return "[]"
	}
	return out
}

func walkJSON(v any, fn func(map[string]any)) {
	switch t := v.(type) {
	case map[string]any:
		fn(t)
		if link, ok := t["link"].(map[string]any); ok {
			fn(link)
		}
		for _, child := range t {
			walkJSON(child, fn)
		}
	case []any:
		for _, child := range t {
			walkJSON(child, fn)
		}
	}
}

func remapLinkIDs(m map[string]any, idMap map[uint]uint) {
	articleish := strings.EqualFold(util.ToString(m["type"]), "article") || isNewsDetailPath(util.ToString(m["path"]))
	if !articleish {
		return
	}
	if id := uintFrom(m["id"]); id > 0 {
		if nid, ok := idMap[id]; ok {
			m["id"] = nid
		}
	}
	query, ok := m["query"].(map[string]any)
	if !ok {
		return
	}
	qid := uintFrom(query["id"])
	if qid == 0 {
		return
	}
	if nid, ok := idMap[qid]; ok {
		query["id"] = nid
	}
}

func rewritePCLink(m map[string]any, resolve func(uint) uint) {
	path := util.ToString(m["path"])
	if path == "" || (!strings.HasPrefix(path, "/pages/") && !strings.HasPrefix(path, "/packages/")) {
		return
	}
	id := uintFrom(m["id"])
	if query, ok := m["query"].(map[string]any); ok {
		if qid := uintFrom(query["id"]); qid > 0 {
			id = qid
		}
	}
	if resolve != nil && id > 0 && isNewsDetailPath(path) {
		id = resolve(id)
		if query, ok := m["query"].(map[string]any); ok {
			query["id"] = id
		}
		m["id"] = id
	}
	q := url.Values{}
	if id > 0 {
		q.Set("id", util.ToString(id))
	}
	if query, ok := m["query"].(map[string]any); ok {
		if typ := util.ToString(query["type"]); typ != "" {
			q.Set("type", typ)
		}
	}
	internal := pcshop.Internal(path, q)
	if internal != "" {
		m["path"] = internal
		delete(m, "query")
	}
}

func isNewsDetailPath(path string) bool {
	return strings.TrimSuffix(path, "/") == "/pages/news_detail/news_detail"
}

func uintFrom(v any) uint {
	n := util.ToInt(v)
	if n < 0 {
		return 0
	}
	return uint(n)
}

func isBannerWidget(name string) bool {
	return name == "banner" || name == "pc-banner"
}

func bannerSlideLinksByImage(raw string) map[string]map[string]any {
	out := map[string]map[string]any{}
	v, ok := decodeJSONValue(raw)
	if !ok {
		return out
	}
	widgets, _ := v.([]any)
	for _, w := range widgets {
		m, _ := w.(map[string]any)
		if !isBannerWidget(util.ToString(m["name"])) {
			continue
		}
		content, _ := m["content"].(map[string]any)
		data, _ := content["data"].([]any)
		for _, item := range data {
			im, _ := item.(map[string]any)
			img := util.ToString(im["image"])
			link, _ := im["link"].(map[string]any)
			if img == "" || len(link) == 0 || util.ToString(link["path"]) == "" {
				continue
			}
			out[imageKey(img)] = cloneMap(link)
		}
	}
	return out
}

func applyBannerSlideLinks(v any, links map[string]map[string]any, onlyDefault bool) {
	widgets, _ := v.([]any)
	for _, w := range widgets {
		m, _ := w.(map[string]any)
		if !isBannerWidget(util.ToString(m["name"])) {
			continue
		}
		content, _ := m["content"].(map[string]any)
		data, _ := content["data"].([]any)
		for _, item := range data {
			im, _ := item.(map[string]any)
			img := util.ToString(im["image"])
			if img == "" {
				continue
			}
			link, ok := links[imageKey(img)]
			if !ok {
				continue
			}
			if onlyDefault && !defaultBannerLink(im["link"]) {
				continue
			}
			im["link"] = cloneMap(link)
		}
	}
}

func defaultBannerLink(v any) bool {
	link, _ := v.(map[string]any)
	if len(link) == 0 {
		return true
	}
	path := strings.TrimSuffix(util.ToString(link["path"]), "/")
	if path == "" {
		return true
	}
	switch path {
	case "/pages/news/news", "/pages/collection/collection":
		return true
	}
	return false
}

func imageKey(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}
