//go:build windows

package auth

import "testing"

// assertConfigOwnerOnly Windows 的 os.FileMode 只反映只读位。
// 0600 写进去之后 Stat 仍是 0666，不能用来判断 NTFS ACL 是否仅属主可读。
// 真正核对 ACL 要依赖 Windows 安全 API，这里不引入这类依赖，也不用假的权限位冒充检查。
// 密钥轮换和旧 token 失效的断言在 store_test.go，不带 build tag，各平台都会跑。
func assertConfigOwnerOnly(t *testing.T, filePath, dirPath string) {
	t.Helper()
	t.Logf("Windows 上 %s / %s 的访问控制由系统 ACL 决定，不检查 Unix 的 0600/0700", filePath, dirPath)
}
