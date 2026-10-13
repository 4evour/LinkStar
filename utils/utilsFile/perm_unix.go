//go:build unix

package utilsFile

import (
	"fmt"
	"os"
	"path/filepath"
)

// projectConfigDirName 与 auth.ConfigPath "config/authConfig.json" 的父目录同名。
// 写在这里是为了不让 utils 反向依赖 auth。
const projectConfigDirName = "config"

// ProtectConfigFile 把配置文件收成仅属主可读写。
// 父目录只有在明确是项目的 config 目录时才收成 0700：目录名是 config，
// 或路径等于、以该配置目录结尾。通用 JSON 写入也会调用这里，
// 不能把 data/、临时目录或其他父目录一并改掉。
// 收紧失败必须返回：文件已经写好，但其他用户仍可能读到密钥。
func ProtectConfigFile(filePath string) error {
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("收紧配置文件 %s 失败，密钥可能仍可被其他用户读到: %w", filePath, err)
	}
	dir := filepath.Dir(filePath)
	if !isProjectConfigDir(dir) {
		return nil
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("收紧配置目录 %s 失败，密钥可能仍可被其他用户读到: %w", dir, err)
	}
	return nil
}

// isProjectConfigDir 判断目录是不是项目配置目录。
// 只认目录名恰好是 config。config.bak、myconfig、以及更上层的父目录都不算。
func isProjectConfigDir(dir string) bool {
	clean := filepath.Clean(dir)
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return false
	}
	return filepath.Base(clean) == projectConfigDirName
}
