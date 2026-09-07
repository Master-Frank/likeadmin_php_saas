package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func TestLookupLogFields(t *testing.T) {
	spec := Lookup("setting.system.log", "lists")
	if spec.FileName != "系统日志" || len(spec.Fields) != 9 {
		t.Fatalf("%+v", spec)
	}
	if spec.Fields[0].Title != "记录ID" || spec.Fields[1].Key != "action" {
		t.Fatalf("fields %+v", spec.Fields)
	}
}

func TestToRecordsUsesChineseHeaders(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "action": " 查看系统日志列表", "extra": "drop"},
	}
	rec := toRecords(rows, []Field{{Key: "id", Title: "记录ID"}, {Key: "action", Title: "操作"}})
	if len(rec) != 2 || rec[0][0] != "记录ID" || rec[0][1] != "操作" {
		t.Fatalf("%v", rec)
	}
	if rec[1][0] != "1" || rec[1][1] != " 查看系统日志列表" {
		t.Fatalf("row %v", rec[1])
	}
}

func TestLookupCompactName(t *testing.T) {
	spec := Lookup("tenant.tenantadmin", "lists")
	if spec.FileName != "租户用户列表" {
		t.Fatalf("%+v", spec)
	}
}

func TestFormatCellEnums(t *testing.T) {
	if formatCell("channel", 1) != "微信小程序" {
		t.Fatalf("channel %s", formatCell("channel", 1))
	}
	if formatCell("disable", 0) != "正常" || formatCell("disable", 1) != "禁用" {
		t.Fatalf("disable %s %s", formatCell("disable", 0), formatCell("disable", 1))
	}
	if formatCell("pay_status_text", 1) != "已支付" {
		t.Fatalf("pay %s", formatCell("pay_status_text", 1))
	}
	if formatCell("pay_status_text", "已支付") != "已支付" {
		t.Fatalf("already text %s", formatCell("pay_status_text", "已支付"))
	}
	rec := toRecords([]map[string]any{{"channel": 2, "disable": 1}}, []Field{{Key: "channel", Title: "注册来源"}, {Key: "disable", Title: "是否禁用"}})
	if len(rec) != 2 || rec[1][0] != "微信公众号" || rec[1][1] != "禁用" {
		t.Fatalf("%v", rec)
	}
}

func TestExcelLongNumericTab(t *testing.T) {
	if !isLongNumeric("123456789012") {
		t.Fatal("12-digit should be long")
	}
	if isLongNumeric("12345678901") {
		t.Fatal("11-digit should not be long")
	}
	out := applyExcelLongNumbers([][]string{{"id"}, {"123456789012"}, {"abc"}})
	if out[1][0] != "123456789012\t" {
		t.Fatalf("tab suffix %q", out[1][0])
	}
	if out[2][0] != "abc" {
		t.Fatalf("text %q", out[2][0])
	}
}

func TestExportRangeError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSize, oldMax := config.C.Project.Lists.PageSize, config.C.Project.Lists.PageSizeMax
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	t.Cleanup(func() {
		config.C.Project.Lists.PageSize = oldSize
		config.C.Project.Lists.PageSizeMax = oldMax
	})

	ctx := func(raw string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/lists"+raw, nil)
		return c
	}

	if msg := exportRangeError(ctx("?export=2&page_start=999&page_end=999"), 10); msg != "第999页到第999页没有数据，无法导出" {
		t.Fatalf("paged empty %q", msg)
	}
	if msg := exportRangeError(ctx("?export=2&page_type=0"), 0); msg != "没有数据,无法导出" {
		t.Fatalf("unpaged empty %q", msg)
	}
	if msg := exportRangeError(ctx("?export=2&page_start=1&page_end=1"), 10); msg != "" {
		t.Fatalf("has data %q", msg)
	}
}

func TestMaybeIgnoresBodyExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/setting.system.log/lists?page_size=1", bytes.NewBufferString(`{"export":2,"file_name":"hack"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists", App: "platformapi"})
	if Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("body export=2 must be ignored")
	}
}

func TestMaybeQueryFileName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?export=1&file_name=自定义导出", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists"})
	c.Set("likeadmin.export_count", int64(3))
	if !Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("export=1")
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data["file_name"] != "自定义导出" {
		t.Fatalf("file_name %v", env.Data["file_name"])
	}
}

func TestWriteXLSXZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.xlsx")
	if err := writeXLSX(path, [][]string{{"记录ID", "操作"}, {"1", "查看"}}); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	seen := map[string]bool{}
	var sheet string
	for _, f := range r.File {
		seen[f.Name] = true
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, 4096)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			sheet = string(buf[:n])
		}
	}
	for _, name := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml"} {
		if !seen[name] {
			t.Fatalf("missing %s", name)
		}
	}
	if !strings.Contains(sheet, "记录ID") || !strings.Contains(sheet, "查看") {
		t.Fatalf("sheet %s", sheet)
	}
}
