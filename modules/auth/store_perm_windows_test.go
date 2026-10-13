//go:build windows

package auth

import "testing"

// assertConfigOwnerOnly Windows 的 os.FileMode 只反映只读位。
// 0600 写进去之后 Stat 仍是 0666，不能用来判断 NTFS ACL 是否仅属主可读。
// 密钥轮换的断言留在不带 build tag 的测试里，这里不把 Unix 权限位当失败。
func assertConfigOwnerOnly(t *testing.T, filePath, dirPath string) {
	t.Helper()
	t.Logf("Windows 上 %s / %s 的访问控制由系统 ACL 决定，不检查 Unix 的 0600/0700", filePath, dirPath)
}
