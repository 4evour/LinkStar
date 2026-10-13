//go:build windows

package utilsFile

// ProtectConfigFile 在 Windows 上不改 NTFS ACL。
//
// os.Chmod 只能切换只读属性，0600 写进去后 Stat 仍是 0666，并不能表示「仅属主」。
// 文件能被谁打开，取决于创建时所在目录的 ACL。核对或改写 ACL 需要额外的 Windows API，
// 这里不引入这类依赖，也不用 Chmod 的成功冒充「仅属主可读」。
// 返回 nil，避免每次保存配置都因做不到的 Unix 权限位而失败；父目录同样不动。
func ProtectConfigFile(filePath string) error {
	return nil
}
