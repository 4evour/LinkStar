//go:build windows

package utilsFile

// ProtectConfigFile 在 Windows 上不改 NTFS ACL。
//
// os.Chmod 只能切换只读属性，0600 写进去后 Stat 仍是 0666，并不能表示「仅属主」。
// 文件能被谁打开，取决于创建时所在目录的 ACL。这里返回 nil，避免每次保存配置都因
// 做不到的 Unix 权限位而失败；也不要把 Chmod 的成功误当成已经收成仅属主可读。
func ProtectConfigFile(filePath string) error {
	return nil
}
