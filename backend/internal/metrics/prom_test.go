package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWritePrometheus(t *testing.T) {
	AddHTTP()
	AddSQL()
	AddExport("ready")
	w := httptest.NewRecorder()
	WritePrometheus(w)
	body := w.Body.String()
	if w.Code != http.StatusOK && w.Code != 0 {
		t.Fatalf("code %d", w.Code)
	}
	for _, want := range []string{"likeadmin_http_requests_total", "likeadmin_sql_queries_total", "likeadmin_export_tasks_total", "status=\"ready\""} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}
