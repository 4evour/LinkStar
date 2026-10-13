//go:build unix

package auth

import (
	"os"
	"testing"
)

// assertConfigOwnerOnly 在支持 Unix 权限位的系统上核对 0600/0700。
func assertConfigOwnerOnly(t *testing.T, filePath, dirPath string) {
	t.Helper()
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("配置文件权限 = %o，想要 600", fileInfo.Mode().Perm())
	}
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("config 目录权限 = %o，想要 700", dirInfo.Mode().Perm())
	}
}
