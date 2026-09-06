package install

import "testing"

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
