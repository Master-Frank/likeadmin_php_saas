package cfgsvc

import (
	"testing"

	"likeadmin/backend/internal/config"
)

func TestGetStringEmptyDefUsesProjectFallback(t *testing.T) {
	config.C.Project.Platform = map[string]string{"name": "SaaS平台端"}
	got := GetString(nil, "platform", "name", "")
	if got != "SaaS平台端" {
		t.Fatalf("got %q", got)
	}
	got = GetString(nil, "platform", "name", "caller-default")
	if got != "caller-default" {
		t.Fatalf("explicit default should win, got %q", got)
	}
}
