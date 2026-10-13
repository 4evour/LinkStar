//go:build unix

package utilsFile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadJsonFile不修改权限(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadJsonFile[map[string]int](path); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o644 {
		t.Fatalf("读取后文件权限 = %o，不应被改掉", fileInfo.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o755 {
		t.Fatalf("读取后目录权限 = %o，不应被改掉", dirInfo.Mode().Perm())
	}
}

func Test收紧不存在的配置文件会报错(t *testing.T) {
	err := ProtectConfigFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "收紧配置文件") {
		t.Fatalf("应收紧失败并带上上下文，实际 %v", err)
	}
}
