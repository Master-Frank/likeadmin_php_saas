package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pairVal(pairs []envPair, key string) (string, bool) {
	for _, p := range pairs {
		if p.Key == key {
			return p.Val, true
		}
	}
	return "", false
}

func TestParseINIPHPScanner(t *testing.T) {
	raw := "[DATABASE]\nDEBUG = true\nCHARSET = utf8mb4\n[PROJECT]\nDEMO_ENV = false\n"
	got := parseINI(raw)
	if v, ok := pairVal(got, "DATABASE.DEBUG"); !ok || v != "1" {
		t.Fatalf("DEBUG true => %q ok=%v", v, ok)
	}
	if v, ok := pairVal(got, "DATABASE.CHARSET"); !ok || v != "utf8mb4" {
		t.Fatalf("CHARSET => %q", v)
	}
	if v, ok := pairVal(got, "PROJECT.DEMO_ENV"); !ok || v != "" {
		t.Fatalf("DEMO_ENV false => %q ok=%v", v, ok)
	}
}

func TestParseINIRealExampleEnv(t *testing.T) {
	raw, err := os.ReadFile("/workspace/server/.example.env")
	if err != nil {
		t.Skip(err)
	}
	got := parseINI(string(raw))
	if v, ok := pairVal(got, "DATABASE.DEBUG"); !ok || v != "1" {
		t.Fatalf("example DEBUG => %q ok=%v", v, ok)
	}
	if v, ok := pairVal(got, "PROJECT.DEMO_ENV"); !ok || v != "" {
		t.Fatalf("example DEMO_ENV => %q ok=%v", v, ok)
	}
	if v, ok := pairVal(got, "PROJECT.DEFAULT_PASSWORD"); !ok || v != "123456" {
		t.Fatalf("example DEFAULT_PASSWORD => %q", v)
	}
	if v, ok := pairVal(got, "LANG.default_lang"); !ok || v != "zh-cn" {
		t.Fatalf("example LANG.default_lang => %q", v)
	}
}

func TestFormatPHPEnvSections(t *testing.T) {
	out := formatPHPEnv([]envPair{
		{Key: "APP_DEBUG", Val: "false"},
		{Key: "APP.DEFAULT_TIMEZONE", Val: "Asia/Shanghai"},
		{Key: "DATABASE.HOSTNAME", Val: "127.0.0.1"},
		{Key: "DATABASE.DEBUG", Val: "1"},
		{Key: "PROJECT.UNIQUE_IDENTIFICATION", Val: "likeadmin"},
		{Key: "HTTP_HOST", Val: "pair1.likeadmin.test"},
	})
	if !strings.Contains(out, "APP_DEBUG = \"false\"") {
		t.Fatalf("missing top-level APP_DEBUG:\n%s", out)
	}
	if !strings.Contains(out, "[DATABASE]") || !strings.Contains(out, "HOSTNAME = \"127.0.0.1\"") {
		t.Fatalf("missing DATABASE section:\n%s", out)
	}
	if !strings.Contains(out, "[PROJECT]") {
		t.Fatalf("missing PROJECT section:\n%s", out)
	}
	if !strings.Contains(out, "HTTP_HOST = \"pair1.likeadmin.test\"") {
		t.Fatalf("missing top-level HTTP_HOST:\n%s", out)
	}
}

func TestWriteEnvMergesExample(t *testing.T) {
	dir := t.TempDir()
	example := filepath.Join(dir, ".example.env")
	src := "APP_DEBUG = false\n\n[APP]\nDEFAULT_TIMEZONE = Asia/Shanghai\n\n[DATABASE]\nTYPE = mysql\nHOSTNAME = 127.0.0.1\nDATABASE = \nUSERNAME = root\nPASSWORD = \nHOSTPORT = 3306\nCHARSET = utf8mb4\nDEBUG = true\nPREFIX = la_\n\n[PROJECT]\nUNIQUE_IDENTIFICATION = likeadmin\nDEFAULT_PASSWORD = 123456\nDEMO_ENV = false\n"
	if err := os.WriteFile(example, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_EXAMPLE_ENV", example)

	envPath := filepath.Join(dir, ".env")
	if err := WriteEnv(envPath, "db.example.test", "likeadmin", "la", "secret", 3307, "la_", "pair1.likeadmin.test", "ab12"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, want := range []string{
		"HOSTNAME = \"db.example.test\"",
		"HOSTPORT = \"3307\"",
		"DATABASE = \"likeadmin\"",
		"USERNAME = \"la\"",
		"PASSWORD = \"secret\"",
		"PREFIX = \"la_\"",
		"DEFAULT_PASSWORD = \"123456\"",
		"UNIQUE_IDENTIFICATION = \"ab12\"",
		"HTTP_HOST = \"pair1.likeadmin.test\"",
		"DEBUG = \"1\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "DEBUG = \"true\"") {
		t.Fatalf("PHP scanner should store DEBUG as 1:\n%s", got)
	}
}

func TestWriteEnvTouchesThenWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", ".env")
	if err := WriteEnv(path, "127.0.0.1", "db", "u", "p", 3306, "la_", "h", "salt"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		t.Fatalf("env not written: %v size=%d", err, st.Size())
	}
}

func TestDefaultEnvPairsHasPHPKeys(t *testing.T) {
	got := defaultEnvPairs()
	need := []string{"APP_DEBUG", "APP.DEFAULT_TIMEZONE", "DATABASE.TYPE", "DATABASE.CHARSET", "PROJECT.UNIQUE_IDENTIFICATION", "PROJECT.DEFAULT_PASSWORD", "PROJECT.DEMO_ENV", "LANG.default_lang"}
	for _, k := range need {
		if _, ok := pairVal(got, k); !ok {
			t.Fatalf("missing default key %s", k)
		}
	}
}

func TestItoa(t *testing.T) {
	if itoa(3307) != "3307" || itoa(0) != "0" || itoa(-2) != "-2" {
		t.Fatalf("itoa %s %s %s", itoa(3307), itoa(0), itoa(-2))
	}
}
