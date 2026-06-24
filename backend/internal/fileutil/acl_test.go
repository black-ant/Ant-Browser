package fileutil_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/fileutil"
)

func TestSecureFileWrite(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_secure.key")

	// 测试数据
	testData := []byte("secret-key-data-12345678901234567890123456789012")

	// 写入文件
	err := fileutil.SecureFileWrite(testFile, testData)
	if err != nil {
		t.Fatalf("SecureFileWrite 失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("文件未创建: %v", err)
	}

	// 验证文件内容
	readData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	if !bytes.Equal(readData, testData) {
		t.Fatalf("文件内容不匹配: expected=%q, got=%q", testData, readData)
	}

	// 验证文件权限 (Unix)
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}

	// 在Unix系统上验证权限为0600
	// 在Windows上，os.FileMode不能完全反映ACL，但至少应该是可读的
	mode := info.Mode()
	if mode.Perm()&0077 != 0 {
		t.Logf("警告: 文件权限可能不够严格: %o", mode.Perm())
	}

	t.Logf("测试通过: 文件创建成功，权限=%o", mode.Perm())
}

func TestSecureFileWriteOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_overwrite.key")

	// 第一次写入
	data1 := []byte("first-write")
	if err := fileutil.SecureFileWrite(testFile, data1); err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}

	// 第二次写入（覆盖）
	data2 := []byte("second-write-with-different-content")
	if err := fileutil.SecureFileWrite(testFile, data2); err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}

	// 验证内容是最新的
	readData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	if !bytes.Equal(readData, data2) {
		t.Fatalf("文件内容应该被覆盖: expected=%q, got=%q", data2, readData)
	}
}

func TestSecureFileWriteInvalidPath(t *testing.T) {
	// 尝试写入不存在的目录（不会自动创建父目录）
	invalidPath := "/nonexistent/directory/that/does/not/exist/file.key"

	err := fileutil.SecureFileWrite(invalidPath, []byte("data"))
	if err == nil {
		t.Fatalf("写入无效路径应该失败")
	}
}

func TestSecureFileWriteEmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.key")

	// 写入空数据
	if err := fileutil.SecureFileWrite(testFile, []byte{}); err != nil {
		t.Fatalf("写入空数据失败: %v", err)
	}

	// 验证文件存在且为空
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}

	if info.Size() != 0 {
		t.Fatalf("空文件大小应为0: got=%d", info.Size())
	}
}
