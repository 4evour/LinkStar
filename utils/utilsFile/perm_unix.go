//go:build unix

package utilsFile

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProtectConfigFile 把配置文件收成仅属主可读写，目录收成仅属主可进入。
// 收紧失败必须返回：文件已经写好，但其他用户仍可能读到密钥。
func ProtectConfigFile(filePath string) error {
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("收紧配置文件 %s 失败，密钥可能仍可被其他用户读到: %w", filePath, err)
	}
	dir := filepath.Dir(filePath)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("收紧配置目录 %s 失败，密钥可能仍可被其他用户读到: %w", dir, err)
	}
	return nil
}
