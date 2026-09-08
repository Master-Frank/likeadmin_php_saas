package cron

import (
	"testing"

	"likeadmin/backend/internal/config"
)

func TestHTTPAllowed(t *testing.T) {
	old := config.C.App.Debug
	t.Cleanup(func() { config.C.App.Debug = old })

	t.Setenv(crontabTokenEnv, "")
	config.C.App.Debug = true
	if !HTTPAllowed("", "") {
		t.Fatal("debug without token should stay PHP-wget compatible")
	}
	config.C.App.Debug = false
	if HTTPAllowed("", "") {
		t.Fatal("non-debug without token must not expose GET /crontab")
	}

	t.Setenv(crontabTokenEnv, "s3cret")
	config.C.App.Debug = false
	if !HTTPAllowed("s3cret", "") {
		t.Fatal("header token")
	}
	if !HTTPAllowed("", "s3cret") {
		t.Fatal("query token")
	}
	if HTTPAllowed("nope", "") {
		t.Fatal("wrong token")
	}
}
