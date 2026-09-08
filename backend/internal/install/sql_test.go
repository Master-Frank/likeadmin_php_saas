package install

import (
	"strings"
	"testing"
)

func TestSplitSQL(t *testing.T) {
	raw := "CREATE TABLE `la_foo` (`id` int);\nINSERT INTO `la_foo` VALUES (1);\n"
	got := SplitSQL(raw)
	if len(got) != 2 {
		t.Fatalf("got %d stmts %#v", len(got), got)
	}
	if rewritePrefix(got[0], "xx_") != "CREATE TABLE `xx_foo` (`id` int)" {
		t.Fatalf("prefix %s", rewritePrefix(got[0], "xx_"))
	}
}

func TestSplitSQLSkipsComments(t *testing.T) {
	raw := "-- demo dump;\nCREATE TABLE `la_foo` (`id` int);\nINSERT INTO `la_foo` VALUES (1);\n"
	got := SplitSQL(raw)
	if len(got) != 2 {
		t.Fatalf("got %d %#v", len(got), got)
	}
}

func TestQualifyInstallSQL(t *testing.T) {
	stmt := "CREATE TABLE `la_foo` (`id` int)"
	got := qualifyInstallSQL(stmt, "likeadmin", "xx_")
	if got != "CREATE TABLE likeadmin.`xx_foo` (`id` int)" {
		t.Fatalf("got %s", got)
	}
	if qualifyInstallSQL(stmt, "", "la_") != stmt {
		t.Fatalf("empty db keeps stmt")
	}
	if qualifyInstallSQL(stmt, "likeadmin", "la_") != "CREATE TABLE likeadmin.`la_foo` (`id` int)" {
		t.Fatalf("same prefix: %s", qualifyInstallSQL(stmt, "likeadmin", "la_"))
	}
}

func TestReadLikeSQLEmbedFallback(t *testing.T) {
	raw, err := ReadLikeSQL(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "CREATE TABLE `la_dev_crontab`") {
		t.Fatal("embed fallback missing like.sql")
	}
}
