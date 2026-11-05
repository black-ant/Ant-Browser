package browser

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isCRXExtensionPackage(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte("Cr24"))
}

func extensionIDFromCRX(data []byte) string {
	if len(data) >= 8 && isCRXExtensionPackage(data) && binary.LittleEndian.Uint32(data[4:8]) == 3 {
		if declaredID := crx3DeclaredID(data); len(declaredID) == 16 {
			return extensionIDFromBytes(declaredID)
		}
	}

	publicKey := crxPublicKey(data)
	if len(publicKey) == 0 {
		return ""
	}
	return extensionIDFromPublicKey(publicKey)
}

func extensionIDFromPublicKey(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return extensionIDFromBytes(hash[:16])
}

func extensionIDFromBytes(rawID []byte) string {
	if len(rawID) != 16 {
		return ""
	}
	var extensionID strings.Builder
	for _, value := range rawID {
		for _, nibble := range []byte{value >> 4, value & 0x0f} {
			if nibble < 10 {
				extensionID.WriteByte(byte('a' + nibble))
			} else {
				extensionID.WriteByte(byte('k' + nibble - 10))
			}
		}
	}
	return extensionID.String()
}

func crx3DeclaredID(data []byte) []byte {
	if len(data) < 12 || !isCRXExtensionPackage(data) || binary.LittleEndian.Uint32(data[4:8]) != 3 {
		return nil
	}
	headerLength := binary.LittleEndian.Uint32(data[8:12])
	headerStart := uint64(12)
	headerEnd := headerStart + uint64(headerLength)
	if headerEnd > uint64(len(data)) {
		return nil
	}
	return crx3DeclaredIDFromHeader(data[headerStart:headerEnd])
}

func crx3DeclaredIDFromHeader(header []byte) []byte {
	for offset := 0; offset < len(header); {
		fieldNumber, wireType, nextOffset, ok := readCRX3FieldTag(header, offset)
		if !ok {
			return nil
		}
		switch wireType {
		case 0:
			_, offset, ok = readCRX3Varint(header, nextOffset)
			if !ok {
				return nil
			}
		case 1:
			offset = nextOffset + 8
		case 2:
			valueLength, valueOffset, valueOK := readCRX3Varint(header, nextOffset)
			if !valueOK || valueLength > uint64(len(header)-valueOffset) {
				return nil
			}
			value := header[valueOffset : valueOffset+int(valueLength)]
			if fieldNumber == 10000 {
				if declaredID := crx3SignedHeaderID(value); len(declaredID) == 16 {
					return declaredID
				}
			}
			offset = valueOffset + int(valueLength)
		case 5:
			offset = nextOffset + 4
		default:
			return nil
		}
		if offset > len(header) {
			return nil
		}
	}
	return nil
}

func crx3SignedHeaderID(signedHeaderData []byte) []byte {
	for offset := 0; offset < len(signedHeaderData); {
		fieldNumber, wireType, nextOffset, ok := readCRX3FieldTag(signedHeaderData, offset)
		if !ok {
			return nil
		}
		if wireType == 2 {
			valueLength, valueOffset, valueOK := readCRX3Varint(signedHeaderData, nextOffset)
			if !valueOK || valueLength > uint64(len(signedHeaderData)-valueOffset) {
				return nil
			}
			if fieldNumber == 1 && valueLength == 16 {
				return append([]byte(nil), signedHeaderData[valueOffset:valueOffset+int(valueLength)]...)
			}
			offset = valueOffset + int(valueLength)
			continue
		}
		if wireType == 0 {
			_, offset, ok = readCRX3Varint(signedHeaderData, nextOffset)
			if !ok {
				return nil
			}
			continue
		}
		return nil
	}
	return nil
}

func crxPublicKey(data []byte) []byte {
	if len(data) < 16 || !isCRXExtensionPackage(data) {
		return nil
	}
	version := binary.LittleEndian.Uint32(data[4:8])
	switch version {
	case 2:
		publicKeyLength := binary.LittleEndian.Uint32(data[8:12])
		signatureLength := binary.LittleEndian.Uint32(data[12:16])
		publicKeyStart := uint64(16)
		publicKeyEnd := publicKeyStart + uint64(publicKeyLength)
		if publicKeyEnd > uint64(len(data)) || publicKeyEnd+uint64(signatureLength) > uint64(len(data)) {
			return nil
		}
		return append([]byte(nil), data[publicKeyStart:publicKeyEnd]...)
	case 3:
		if len(data) < 12 {
			return nil
		}
		headerLength := binary.LittleEndian.Uint32(data[8:12])
		headerStart := uint64(12)
		headerEnd := headerStart + uint64(headerLength)
		if headerEnd > uint64(len(data)) {
			return nil
		}
		return crx3PublicKeyFromHeader(data[headerStart:headerEnd])
	default:
		return nil
	}
}

func crxPublicKeys(data []byte) [][]byte {
	if len(data) < 8 || !isCRXExtensionPackage(data) {
		return nil
	}
	version := binary.LittleEndian.Uint32(data[4:8])
	if version == 2 {
		publicKey := crxPublicKey(data)
		if len(publicKey) == 0 {
			return nil
		}
		return [][]byte{publicKey}
	}
	if version != 3 || len(data) < 12 {
		return nil
	}
	headerLength := binary.LittleEndian.Uint32(data[8:12])
	headerStart := uint64(12)
	headerEnd := headerStart + uint64(headerLength)
	if headerEnd > uint64(len(data)) {
		return nil
	}
	return crx3PublicKeysFromHeader(data[headerStart:headerEnd])
}

func crx3PublicKeyFromHeader(header []byte) []byte {
	keys := crx3PublicKeysFromHeader(header)
	if len(keys) == 0 {
		return nil
	}
	return keys[0]
}

func crx3PublicKeysFromHeader(header []byte) [][]byte {
	keys := make([][]byte, 0)
	for offset := 0; offset < len(header); {
		fieldNumber, wireType, nextOffset, ok := readCRX3FieldTag(header, offset)
		if !ok {
			return nil
		}
		switch wireType {
		case 0:
			_, offset, ok = readCRX3Varint(header, nextOffset)
			if !ok {
				return nil
			}
		case 1:
			offset = nextOffset + 8
		case 2:
			valueLength, valueOffset, valueOK := readCRX3Varint(header, nextOffset)
			if !valueOK || valueLength > uint64(len(header)-valueOffset) {
				return nil
			}
			value := header[valueOffset : valueOffset+int(valueLength)]
			if fieldNumber == 2 || fieldNumber == 3 {
				if publicKey := crx3ProofPublicKey(value); len(publicKey) > 0 {
					keys = append(keys, publicKey)
				}
			}
			offset = valueOffset + int(valueLength)
		case 5:
			offset = nextOffset + 4
		default:
			return nil
		}
		if offset > len(header) {
			return keys
		}
	}
	return keys
}

func crx3ProofPublicKey(proof []byte) []byte {
	for offset := 0; offset < len(proof); {
		fieldNumber, wireType, nextOffset, ok := readCRX3FieldTag(proof, offset)
		if !ok {
			return nil
		}
		if wireType == 2 {
			valueLength, valueOffset, valueOK := readCRX3Varint(proof, nextOffset)
			if !valueOK || valueLength > uint64(len(proof)-valueOffset) {
				return nil
			}
			if fieldNumber == 1 {
				return append([]byte(nil), proof[valueOffset:valueOffset+int(valueLength)]...)
			}
			offset = valueOffset + int(valueLength)
			continue
		}
		if wireType == 0 {
			_, offset, ok = readCRX3Varint(proof, nextOffset)
			if !ok {
				return nil
			}
			continue
		}
		return nil
	}
	return nil
}

func readCRX3FieldTag(data []byte, offset int) (uint64, byte, int, bool) {
	tag, nextOffset, ok := readCRX3Varint(data, offset)
	if !ok || tag < 8 {
		return 0, 0, 0, false
	}
	return tag >> 3, byte(tag & 0x07), nextOffset, true
}

func readCRX3Varint(data []byte, offset int) (uint64, int, bool) {
	var value uint64
	for index := 0; index < 10 && offset+index < len(data); index++ {
		current := data[offset+index]
		value |= uint64(current&0x7f) << (7 * index)
		if current&0x80 == 0 {
			return value, offset + index + 1, true
		}
	}
	return 0, 0, false
}

func (m *Manager) storeExtensionPackage(extensionID string, data []byte) (string, string, error) {
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return "", "", fmt.Errorf("插件 ID 不能为空")
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("插件包为空")
	}

	hash := sha256.Sum256(data)
	packageHash := hex.EncodeToString(hash[:])
	packageRoot := m.ResolveRelativePath(filepath.Join("data", extensionsRootDir, "packages"))
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		return "", "", fmt.Errorf("创建插件包目录失败: %w", err)
	}
	packagePath := filepath.Join(packageRoot, extensionID+".crx")
	if existing, err := os.ReadFile(packagePath); err == nil {
		existingHash := sha256.Sum256(existing)
		if hex.EncodeToString(existingHash[:]) == packageHash {
			return packagePath, packageHash, nil
		}
	}

	tempPath := fmt.Sprintf("%s.tmp-%d", packagePath, time.Now().UnixNano())
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("保存插件包失败: %w", err)
	}
	if err := os.Rename(tempPath, packagePath); err != nil {
		_ = os.Remove(tempPath)
		return "", "", fmt.Errorf("替换插件包失败: %w", err)
	}
	return packagePath, packageHash, nil
}

func extensionPackageHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func runtimeExtensionIDFromPackage(packagePath string, extension Extension) (string, error) {
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("读取插件包失败: %w", err)
	}
	if runtimeID := extensionIDFromCRX(data); runtimeID != "" {
		return runtimeID, nil
	}
	if runtimeID := NormalizeExtensionID(extension.ExtensionID); runtimeID != "" {
		return runtimeID, nil
	}
	return "", fmt.Errorf("无法从插件包识别运行时 ID")
}
