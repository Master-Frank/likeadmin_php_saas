package filesvc

import (
	"regexp"
	"strings"
	"testing"

	"likeadmin/backend/internal/util"
)

func TestBuildSaveName(t *testing.T) {
	got := buildSaveName("/tmp/likeadmin-real-path", "png")
	if !regexp.MustCompile(`^\d{14}[0-9a-f]{5}\d{4}\.png$`).MatchString(got) {
		t.Fatalf("shape %s", got)
	}
	want := util.MD5("/tmp/likeadmin-real-path")[:5]
	if got[14:19] != want {
		t.Fatalf("hash %s want %s", got[14:19], want)
	}
	cased := buildSaveName("/tmp/likeadmin-real-path", "PNG")
	if !strings.HasSuffix(cased, ".PNG") {
		t.Fatalf("PHP pathinfo keeps extension case: %s", cased)
	}
}
