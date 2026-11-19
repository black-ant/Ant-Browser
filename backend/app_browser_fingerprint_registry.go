package backend

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// 内置指纹能力表：随安装包分发，作为“哪个内核版本支持哪些参数”的唯一数据源。
// 修改规则时只改 JSON，不要在代码里再复制一份版本判断。
//
//go:embed fingerprint_capabilities.json
var fingerprintCapabilityData []byte

const (
	fingerprintSupportSupported             = "supported"
	fingerprintSupportSupportedMeasured     = "supported_measured"
	fingerprintSupportSupportedDocumented   = "supported_documented"
	fingerprintSupportSupportedPolicy       = "supported_policy"
	fingerprintSupportUnsupported           = "unsupported"
	fingerprintSupportUnsupportedMeasured   = "unsupported_measured"
	fingerprintSupportUnsupportedDocumented = "unsupported_documented"
	fingerprintSupportUnsupportedPolicy     = "unsupported_policy"
	fingerprintSupportConverted             = "converted"
	fingerprintSupportUnknown               = "unknown"
)

type BrowserFingerprintCapabilityTable struct {
	SchemaVersion     int                                   `json:"schemaVersion"`
	UpdatedAt         string                                `json:"updatedAt"`
	Notes             []string                              `json:"notes"`
	CurrentVersionKey string                                `json:"currentVersionKey"`
	CurrentMajor      int                                   `json:"currentMajor"`
	Versions          []BrowserFingerprintCapabilityVersion `json:"versions"`
	Groups            []BrowserFingerprintCapabilityGroup   `json:"groups"`
	Warnings          []string                              `json:"warnings"`
}

type BrowserFingerprintCapabilityVersion struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	MinMajor int    `json:"minMajor"`
	MaxMajor int    `json:"maxMajor"`
	Source   string `json:"source"`
	Note     string `json:"note"`
}

type BrowserFingerprintCapabilityGroup struct {
	Key   string                             `json:"key"`
	Label string                             `json:"label"`
	Items []BrowserFingerprintCapabilityItem `json:"items"`
}

type BrowserFingerprintCapabilityItem struct {
	Arg        string            `json:"arg"`
	Label      string            `json:"label"`
	Sample     string            `json:"sample"`
	RuntimeArg string            `json:"runtimeArg"`
	Support    map[string]string `json:"support"`
	Note       string            `json:"note"`
}

var (
	fingerprintCapabilityOnce sync.Once
	fingerprintCapabilityRaw  BrowserFingerprintCapabilityTable
	fingerprintCapabilityErr  error
)

// BrowserProfileFingerprintCapabilityTable 返回内置能力表，并标注当前实例内核命中的版本段。
func (a *App) BrowserProfileFingerprintCapabilityTable(profileId string, coreId string) BrowserFingerprintCapabilityTable {
	table, err := loadFingerprintCapabilityTable()
	if err != nil {
		return BrowserFingerprintCapabilityTable{Warnings: []string{err.Error()}}
	}

	chromeVersion, found := a.fingerprintReportChromeVersion(profileId, coreId)
	major := parseChromeMajor(chromeVersion)
	table.CurrentMajor = major
	if !found {
		table.Warnings = appendUniqueString(table.Warnings, "未找到可用内核，能力表不标注当前内核")
		return table
	}
	if major <= 0 {
		table.Warnings = appendUniqueString(table.Warnings, "未识别当前内核版本，按未知版本保守处理")
		return table
	}
	table.CurrentVersionKey = matchFingerprintCapabilityVersion(table.Versions, major)
	if table.CurrentVersionKey == "" {
		table.Warnings = appendUniqueString(table.Warnings, fmt.Sprintf("当前内核主版本 %d 未收录在能力表中，按未知处理", major))
	}
	return table
}

func (a *App) fingerprintReportChromeVersion(profileId string, coreId string) (string, bool) {
	profileId = strings.TrimSpace(profileId)
	coreId = strings.TrimSpace(coreId)
	if a == nil || a.browserMgr == nil {
		return "", false
	}
	if coreId == "" && profileId != "" {
		a.browserMgr.Mutex.Lock()
		if profile, ok := a.browserMgr.Profiles[profileId]; ok && profile != nil {
			coreId = strings.TrimSpace(profile.CoreId)
		}
		a.browserMgr.Mutex.Unlock()
	}
	core, found := a.resolveBrowserCoreForFingerprintReport(coreId)
	if !found {
		return "", false
	}
	return a.browserMgr.GetChromeVersion(core.CorePath), true
}

func matchFingerprintCapabilityVersion(versions []BrowserFingerprintCapabilityVersion, major int) string {
	if major <= 0 {
		return ""
	}
	for _, version := range versions {
		if major < version.MinMajor {
			continue
		}
		if version.MaxMajor != 0 && major > version.MaxMajor {
			continue
		}
		return version.Key
	}
	return ""
}

func loadFingerprintCapabilityTable() (BrowserFingerprintCapabilityTable, error) {
	fingerprintCapabilityOnce.Do(func() {
		var table BrowserFingerprintCapabilityTable
		if err := json.Unmarshal(fingerprintCapabilityData, &table); err != nil {
			fingerprintCapabilityErr = fmt.Errorf("解析内置指纹能力表失败: %w", err)
			return
		}
		if err := validateFingerprintCapabilityTable(table); err != nil {
			fingerprintCapabilityErr = err
			return
		}
		fingerprintCapabilityRaw = table
	})
	if fingerprintCapabilityErr != nil {
		return BrowserFingerprintCapabilityTable{}, fingerprintCapabilityErr
	}
	return cloneFingerprintCapabilityTable(fingerprintCapabilityRaw), nil
}

func validateFingerprintCapabilityTable(table BrowserFingerprintCapabilityTable) error {
	if table.SchemaVersion != 1 {
		return fmt.Errorf("指纹能力表版本不受支持: %d", table.SchemaVersion)
	}
	if len(table.Versions) == 0 {
		return fmt.Errorf("指纹能力表缺少内核版本段")
	}
	versionKeys := make(map[string]struct{}, len(table.Versions))
	for _, version := range table.Versions {
		key := strings.TrimSpace(version.Key)
		if key == "" {
			return fmt.Errorf("指纹能力表存在空的版本标识")
		}
		if _, exists := versionKeys[key]; exists {
			return fmt.Errorf("指纹能力表版本标识重复: %s", key)
		}
		if version.MaxMajor != 0 && version.MaxMajor < version.MinMajor {
			return fmt.Errorf("指纹能力表版本区间非法: %s", key)
		}
		versionKeys[key] = struct{}{}
	}

	ranges := append([]BrowserFingerprintCapabilityVersion{}, table.Versions...)
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].MinMajor < ranges[j].MinMajor })
	if ranges[0].MinMajor != 0 {
		return fmt.Errorf("指纹能力表版本区间必须从 0 开始")
	}
	for index := 0; index < len(ranges); index++ {
		if index == len(ranges)-1 {
			if ranges[index].MaxMajor != 0 {
				return fmt.Errorf("指纹能力表版本区间缺少无上限收尾段")
			}
			break
		}
		if ranges[index].MaxMajor == 0 {
			return fmt.Errorf("指纹能力表版本区间存在无限上限但不在末尾: %s", ranges[index].Key)
		}
		if ranges[index].MaxMajor+1 != ranges[index+1].MinMajor {
			return fmt.Errorf("指纹能力表版本区间不连续或重叠: %s -> %s", ranges[index].Key, ranges[index+1].Key)
		}
	}

	seenArgs := make(map[string]struct{})
	for _, group := range table.Groups {
		if strings.TrimSpace(group.Key) == "" {
			return fmt.Errorf("指纹能力表存在空的分组标识")
		}
		for _, item := range group.Items {
			arg := strings.TrimSpace(item.Arg)
			if arg == "" {
				return fmt.Errorf("指纹能力表分组 %s 存在空参数", group.Key)
			}
			if _, exists := seenArgs[arg]; exists {
				return fmt.Errorf("指纹能力表参数重复: %s", arg)
			}
			seenArgs[arg] = struct{}{}
			for key := range versionKeys {
				status, exists := item.Support[key]
				if !exists {
					return fmt.Errorf("参数 %s 缺少版本 %s 的支持状态", arg, key)
				}
				switch status {
				case fingerprintSupportSupported, fingerprintSupportSupportedMeasured, fingerprintSupportSupportedDocumented, fingerprintSupportSupportedPolicy, fingerprintSupportUnsupported, fingerprintSupportUnsupportedMeasured, fingerprintSupportUnsupportedDocumented, fingerprintSupportUnsupportedPolicy, fingerprintSupportConverted, fingerprintSupportUnknown:
				default:
					return fmt.Errorf("参数 %s 的版本 %s 状态非法: %s", arg, key, status)
				}
				if status == fingerprintSupportConverted && strings.TrimSpace(item.RuntimeArg) == "" {
					return fmt.Errorf("参数 %s 在版本 %s 声明为转换，但缺少运行参数", arg, key)
				}
			}
		}
	}
	return nil
}

func cloneFingerprintCapabilityTable(src BrowserFingerprintCapabilityTable) BrowserFingerprintCapabilityTable {
	dst := src
	dst.Notes = append([]string{}, src.Notes...)
	dst.Warnings = append([]string{}, src.Warnings...)
	dst.Versions = append([]BrowserFingerprintCapabilityVersion{}, src.Versions...)
	groups := make([]BrowserFingerprintCapabilityGroup, 0, len(src.Groups))
	for _, group := range src.Groups {
		copied := group
		copied.Items = make([]BrowserFingerprintCapabilityItem, 0, len(group.Items))
		for _, item := range group.Items {
			copiedItem := item
			copiedItem.Support = make(map[string]string, len(item.Support))
			for key, status := range item.Support {
				copiedItem.Support[key] = status
			}
			copied.Items = append(copied.Items, copiedItem)
		}
		groups = append(groups, copied)
	}
	dst.Groups = groups
	return dst
}
