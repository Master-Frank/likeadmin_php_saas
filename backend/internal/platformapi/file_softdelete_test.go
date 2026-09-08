package platformapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func initFileDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	return bootstrap.DB != nil
}

func postJSON(t *testing.T, h gin.HandlerFunc, body map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	h(c)
}

func TestFileRenameSkipsSoftDeleted(t *testing.T) {
	if !initFileDB(t) {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.File{
		Cid: 0, Type: 10, Name: "soft-orig.png", URI: "uploads/pair-soft-orig.png",
		Source: 0, CreateTime: now, UpdateTime: util.UnixPtr(now), DeleteTime: util.UnixPtr(now),
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.File{}) })

	postJSON(t, FileRename, map[string]any{"id": row.ID, "name": "soft-renamed.png"})

	var got model.File
	if bootstrap.DB.Where("id = ?", row.ID).First(&got).Error != nil {
		t.Fatal("row missing")
	}
	if got.Name != "soft-orig.png" {
		t.Fatalf("PHP SoftDelete File::rename must not touch deleted rows, got %q", got.Name)
	}
	if got.DeleteTime == nil {
		t.Fatal("delete_time must stay set")
	}
}

func TestFileMoveSkipsSoftDeleted(t *testing.T) {
	if !initFileDB(t) {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.File{
		Cid: 3, Type: 10, Name: "soft-move.png", URI: "uploads/pair-soft-move.png",
		Source: 0, CreateTime: now, UpdateTime: util.UnixPtr(now), DeleteTime: util.UnixPtr(now),
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.File{}) })

	postJSON(t, FileMove, map[string]any{"ids": []uint{row.ID}, "cid": 99})

	var got model.File
	if bootstrap.DB.Where("id = ?", row.ID).First(&got).Error != nil {
		t.Fatal("row missing")
	}
	if got.Cid != 3 {
		t.Fatalf("PHP SoftDelete File::move must not touch deleted rows, cid=%d", got.Cid)
	}
}

func TestFileRenameUpdatesAlive(t *testing.T) {
	if !initFileDB(t) {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.File{
		Cid: 0, Type: 10, Name: "alive-orig.png", URI: "uploads/pair-alive-orig.png",
		Source: 0, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.File{}) })

	postJSON(t, FileRename, map[string]any{"id": row.ID, "name": "alive-renamed.png"})

	var got model.File
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", row.ID).First(&got).Error != nil {
		t.Fatal("alive row missing")
	}
	if got.Name != "alive-renamed.png" {
		t.Fatalf("alive file should rename, got %q", got.Name)
	}
}
