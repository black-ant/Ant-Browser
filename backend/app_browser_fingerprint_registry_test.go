package backend

import (
	"sort"
	"testing"
)

func TestFingerprintCapabilityTableLoads(t *testing.T) {
	table, err := loadFingerprintCapabilityTable()
	if err != nil {
		t.Fatalf("loadFingerprintCapabilityTable returned error: %v", err)
	}
	if len(table.Versions) == 0 {
		t.Fatal("capability table has no versions")
	}
	if len(table.Groups) == 0 {
		t.Fatal("capability table has no groups")
	}
	itemCount := 0
	for _, group := range table.Groups {
		itemCount += len(group.Items)
	}
	if itemCount == 0 {
		t.Fatal("capability table has no items")
	}
}

func TestMatchFingerprintCapabilityVersion(t *testing.T) {
	table, err := loadFingerprintCapabilityTable()
	if err != nil {
		t.Fatalf("loadFingerprintCapabilityTable returned error: %v", err)
	}
	tests := []struct {
		major int
		want  string
	}{
		{major: 120, want: "v130"},
		{major: 130, want: "v130"},
		{major: 131, want: "v131"},
		{major: 138, want: "v131"},
		{major: 139, want: "v139"},
		{major: 141, want: "v139"},
		{major: 142, want: "v142"},
		{major: 143, want: "v142"},
		{major: 144, want: "v144"},
		{major: 149, want: "v144"},
		{major: 150, want: "v150"},
		{major: 200, want: "v150"},
		{major: 0, want: ""},
	}
	for _, test := range tests {
		if got := matchFingerprintCapabilityVersion(table.Versions, test.major); got != test.want {
			t.Fatalf("matchFingerprintCapabilityVersion(%d) = %q, want %q", test.major, got, test.want)
		}
	}
}

// TestFingerprintCapabilityTableMatchesLaunchPlan 保证能力表与真实启动策略不漂移：
// 表里标支持的参数必须原样传给内核，标不支持的必须不出现，标转换的必须出现新参数。
func TestFingerprintCapabilityTableMatchesLaunchPlan(t *testing.T) {
	table, err := loadFingerprintCapabilityTable()
	if err != nil {
		t.Fatalf("loadFingerprintCapabilityTable returned error: %v", err)
	}
	versions := []struct {
		key           string
		chromeVersion string
	}{
		{key: "v130", chromeVersion: "130.0.6723.117"},
		{key: "v131", chromeVersion: "135.0.7049.95"},
		{key: "v139", chromeVersion: "140.0.7339.82"},
		{key: "v142", chromeVersion: "143.0.7499.41"},
		{key: "v144", chromeVersion: "144.0.7559.132"},
		{key: "v150", chromeVersion: "150.0.7871.186"},
	}
	for _, version := range versions {
		for _, group := range table.Groups {
			for _, item := range group.Items {
				if item.Sample == "" {
					t.Fatalf("参数 %s 缺少示例值", item.Arg)
				}
				plan := buildBrowserFingerprintLaunchPlanForHost("", []string{item.Sample}, version.chromeVersion, "windows")
				status := item.Support[version.key]
				switch status {
				case fingerprintSupportSupported, fingerprintSupportSupportedMeasured, fingerprintSupportSupportedDocumented, fingerprintSupportSupportedPolicy:
					if !fingerprintArgsContain(plan.launchArgs, item.Sample) {
						t.Errorf("%s 在 %s 上应保留参数 %s，实际 %v", item.Arg, version.key, item.Sample, plan.launchArgs)
					}
				case fingerprintSupportUnsupported, fingerprintSupportUnsupportedMeasured, fingerprintSupportUnsupportedPolicy:
					if fingerprintArgsContainKey(plan.launchArgs, item.Arg) {
						t.Errorf("%s 在 %s 上应被移除，实际 %v", item.Arg, version.key, plan.launchArgs)
					}
				case fingerprintSupportConverted:
					if !fingerprintArgsContain(plan.launchArgs, item.RuntimeArg) {
						t.Errorf("%s 在 %s 上应转换为 %s，实际 %v", item.Arg, version.key, item.RuntimeArg, plan.launchArgs)
					}
				default:
					// 文档不支持与未验证状态只描述内核能力，不约束应用是否透传参数。
				}
			}
		}
	}
}

// TestFingerprintCapabilityTableCoversModeledArgs 防止启动策略新增参数后忘记同步能力表。
func TestFingerprintCapabilityTableCoversModeledArgs(t *testing.T) {
	table, err := loadFingerprintCapabilityTable()
	if err != nil {
		t.Fatalf("loadFingerprintCapabilityTable returned error: %v", err)
	}
	known := make(map[string]struct{})
	for _, group := range table.Groups {
		for _, item := range group.Items {
			known[item.Arg] = struct{}{}
		}
	}

	missing := make([]string, 0)
	check := func(keys []string) {
		for _, key := range keys {
			if _, ok := known[key]; !ok {
				missing = append(missing, key)
			}
		}
	}
	for key := range fingerprintStableArgLabels {
		check([]string{key})
	}
	for key := range fingerprintChrome144RemovedArgLabels {
		check([]string{key})
	}
	for key := range fingerprintNoEffectArgLabels {
		check([]string{key})
	}
	for key := range fingerprintChrome144ConvertedArgLabels {
		check([]string{key})
	}
	for key := range fingerprintEffectiveNoiseArgLabels {
		check([]string{key})
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("能力表缺少启动策略已建模的参数: %v", missing)
	}
}

func TestBrowserProfileFingerprintCapabilityTableWithoutCore(t *testing.T) {
	app := &App{}
	table := app.BrowserProfileFingerprintCapabilityTable("", "")
	if len(table.Versions) != 6 {
		t.Fatalf("versions = %d, want 6", len(table.Versions))
	}
	if len(table.Groups) == 0 {
		t.Fatal("groups is empty")
	}
	if table.CurrentVersionKey != "" {
		t.Fatalf("current version = %q, want empty without core", table.CurrentVersionKey)
	}
	if len(table.Warnings) == 0 {
		t.Fatal("warnings is empty without core")
	}
}

func fingerprintArgsContain(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func fingerprintArgsContainKey(args []string, key string) bool {
	for _, arg := range args {
		if browserFingerprintArgKey(arg) == key {
			return true
		}
	}
	return false
}
