package decorate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapTemplateArticleIDsSkipsSameID(t *testing.T) {
	got := MapTemplateArticleIDs(
		[]ArticleIDTitle{{ID: 3, Title: "金山电池"}, {ID: 1, Title: "居家好物"}},
		[]ArticleIDTitle{{ID: 3, Title: "金山电池"}, {ID: 1, Title: "居家好物"}},
	)
	if len(got) != 0 {
		t.Fatalf("same-id copies should not remap, got %v", got)
	}
	got = MapTemplateArticleIDs(
		[]ArticleIDTitle{{ID: 3, Title: "金山电池"}},
		[]ArticleIDTitle{{ID: 6, Title: "金山电池"}},
	)
	if got[3] != 6 {
		t.Fatalf("expected 3→6, got %v", got)
	}
}

func TestRemapArticleIDsRewritesAliasCopyID(t *testing.T) {
	raw := `{"path":"/pages/news_detail/news_detail","query":{"id":6},"type":"article","id":6}`
	got := RemapArticleIDs(raw, map[uint]uint{3: 69, 6: 69})
	if strings.Contains(got, `"id":6,`) || strings.Contains(got, `"id":6}`) {
		t.Fatalf("alias copy id leaked: %s", got)
	}
	if !strings.Contains(got, `"id":69`) {
		t.Fatalf("expected tenant copy 69, got %s", got)
	}
}

func TestRemapArticleIDsRewritesStandaloneTabbarLink(t *testing.T) {
	raw := `{"path":"/pages/news_detail/news_detail","query":{"id":3},"type":"article","id":3}`
	got := RemapArticleIDs(raw, map[uint]uint{3: 6})
	if strings.Contains(got, `"id":3`) {
		t.Fatalf("template id leaked: %s", got)
	}
	if !strings.Contains(got, `"id":6`) {
		t.Fatalf("expected tenant copy id 6, got %s", got)
	}
}

func TestRemapArticleIDsRewritesNewsDetailQuery(t *testing.T) {
	raw := `[{"name":"banner","content":{"data":[{"link":{"path":"/pages/news_detail/news_detail","query":{"id":3},"type":"article","id":3}}]}}]`
	got := RemapArticleIDs(raw, map[uint]uint{3: 6})
	if !strings.Contains(got, `"id":6`) {
		t.Fatalf("expected tenant copy id 6, got %s", got)
	}
	if strings.Contains(got, `"id":3`) {
		t.Fatalf("template id leaked: %s", got)
	}
}

func TestCopyBannerLinksByImageAlignsPCWithMobile(t *testing.T) {
	mobile := `[{"name":"banner","content":{"data":[
		{"image":"/resource/image/tenantapi/default/banner001.png","link":{"path":"/pages/news_detail/news_detail","query":{"id":6},"type":"article","id":6}},
		{"image":"/resource/image/tenantapi/default/banner002.png","link":{"path":"/pages/news_detail/news_detail","query":{"id":3},"type":"article","id":3}}
	]}}]`
	pc := `[{"name":"pc-banner","content":{"data":[
		{"image":"/resource/image/tenantapi/default/banner002.png","link":{"path":"/pages/collection/collection","type":"shop"}},
		{"image":"/resource/image/tenantapi/default/banner001.png","link":{}}
	]}}]`
	got := CopyBannerLinksByImage(pc, mobile)
	if !strings.Contains(got, `"id":3`) || !strings.Contains(got, `"id":6`) {
		t.Fatalf("expected article ids copied, got %s", got)
	}
	if strings.Contains(got, "/pages/collection/collection") {
		t.Fatalf("shop link should be replaced: %s", got)
	}
	kept := `[{"name":"pc-banner","content":{"data":[{"image":"/resource/image/tenantapi/default/banner002.png","link":{"path":"/pages/news_detail/news_detail","query":{"id":9},"type":"article"}}]}}]`
	filled := FillDefaultBannerLinksByImage(kept, mobile)
	if !strings.Contains(filled, `"id":9`) {
		t.Fatalf("custom article link must stay, got %s", filled)
	}
	if strings.Contains(filled, `"id":3`) {
		t.Fatalf("must not overwrite custom article: %s", filled)
	}
}

func TestRewriteForPCMapsShopAndArticle(t *testing.T) {
	raw := `[{"name":"pc-banner","content":{"data":[
		{"link":{"path":"/pages/news/news","type":"shop"}},
		{"link":{"path":"/pages/news_detail/news_detail","query":{"id":3},"type":"article"}}
	]}}]`
	got := RewriteForPC(raw, func(id uint) uint {
		if id == 3 {
			return 6
		}
		return id
	})
	var widgets []map[string]any
	if err := json.Unmarshal([]byte(got), &widgets); err != nil {
		t.Fatal(err)
	}
	data, _ := widgets[0]["content"].(map[string]any)["data"].([]any)
	first, _ := data[0].(map[string]any)["link"].(map[string]any)
	if first["path"] != "/information" {
		t.Fatalf("shop path %v", first["path"])
	}
	second, _ := data[1].(map[string]any)["link"].(map[string]any)
	if second["path"] != "/information/detail/6" {
		t.Fatalf("article path %v", second["path"])
	}
}
