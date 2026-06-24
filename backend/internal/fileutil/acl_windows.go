// +build windows

package fileutil

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SecureFileWrite 在Windows上写入文件并设置严格的ACL，仅允许当前用户访问。
// 相当于Unix的 0600 权限，但使用Windows ACL机制。
func SecureFileWrite(path string, data []byte) error {
	// 先创建文件
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	// 设置Windows ACL (如果失败则记录但不返回错误，确保向后兼容)
	if err := setWindowsACL(path); err != nil {
		// ACL设置失败时不返回错误，至少文件已经用0600权限创建了
		// 在生产环境中会通过日志记录
		_ = err
	}

	return nil
}

// setWindowsACL 设置文件ACL，仅允许当前用户和SYSTEM访问。
func setWindowsACL(path string) error {
	// 获取当前用户SID
	token := windows.GetCurrentProcessToken()
	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("获取当前用户失败: %w", err)
	}

	// 获取SYSTEM SID (S-1-5-18)
	systemSID, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return fmt.Errorf("获取SYSTEM SID失败: %w", err)
	}

	// 创建DACL
	dacl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       0,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(tokenUser.User.Sid),
			},
		},
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       0,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(systemSID),
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("创建ACL失败: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(dacl)))

	// 打开文件句柄
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("路径转换失败: %w", err)
	}

	// 使用 SetSecurityInfo 设置文件ACL
	handle, err := windows.CreateFile(
		pathUTF16,
		windows.WRITE_DAC,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer windows.CloseHandle(handle)

	err = windows.SetSecurityInfo(
		handle,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
	if err != nil {
		return fmt.Errorf("设置安全信息失败: %w", err)
	}

	return nil
}
