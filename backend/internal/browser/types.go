package browser

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
	"os/exec"
	"sync"
)

// Profile 浏览器配置文件
type Profile struct {
	ProfileId          string        `json:"profileId"`
	ProfileName        string        `json:"profileName"`
	UserDataDir        string        `json:"userDataDir"`
	CoreId             string        `json:"coreId"`
	FingerprintArgs    []string      `json:"fingerprintArgs"`
	ProxyId            string        `json:"proxyId"`
	ProxyConfig        string        `json:"proxyConfig"`
	ProxyBindSourceID  string        `json:"proxyBindSourceId"`
	ProxyBindSourceURL string        `json:"proxyBindSourceUrl"`
	ProxyBindName      string        `json:"proxyBindName"`
	ProxyBindUpdatedAt string        `json:"proxyBindUpdatedAt"`
	LaunchArgs         []string      `json:"launchArgs"`
	Tags               []string      `json:"tags"`
	Keywords           []string      `json:"keywords"`
	GroupId            string        `json:"groupId"` // 所属分组ID
	LaunchCode         string        `json:"launchCode"`
	Running            bool          `json:"running"`
	Status             ProfileStatus `json:"status"` // 运行时状态机；与 Running 并存，不替代其语义
	DebugPort          int           `json:"debugPort"`
	DebugReady         bool          `json:"debugReady"`
	Pid                int           `json:"pid"`
	RuntimeWarning     string        `json:"runtimeWarning"`
	LastError          string        `json:"lastError"`
	CreatedAt          string        `json:"createdAt"`
	UpdatedAt          string        `json:"updatedAt"`
	LastStartAt        string        `json:"lastStartAt"`
	LastStopAt         string        `json:"lastStopAt"`
}

// ProfileInput 创建/更新配置文件的输入
type ProfileInput struct {
	ProfileName     string   `json:"profileName"`
	UserDataDir     string   `json:"userDataDir"`
	CoreId          string   `json:"coreId"`
	FingerprintArgs []string `json:"fingerprintArgs"`
	ProxyId         string   `json:"proxyId"`
	ProxyConfig     string   `json:"proxyConfig"`
	LaunchArgs      []string `json:"launchArgs"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	GroupId         string   `json:"groupId"` // 所属分组ID
	LaunchCode      string   `json:"launchCode"`
	AccountIds      []string `json:"accountIds"` // 关联的账号ID列表
}

// Tab 浏览器标签页
type Tab struct {
	TabId  string `json:"tabId"`
	Title  string `json:"title"`
	Url    string `json:"url"`
	Active bool   `json:"active"`
}

// Settings 浏览器全局设置
type Settings struct {
	UserDataRoot           string   `json:"userDataRoot"`
	DefaultFingerprintArgs []string `json:"defaultFingerprintArgs"`
	DefaultLaunchArgs      []string `json:"defaultLaunchArgs"`
	DefaultProxy           string   `json:"defaultProxy"`
	StartReadyTimeoutMs    int      `json:"startReadyTimeoutMs"`
	StartStableWindowMs    int      `json:"startStableWindowMs"`
}

// CoreInput 内核配置输入
type CoreInput struct {
	CoreId    string `json:"coreId"`
	CoreName  string `json:"coreName"`
	CorePath  string `json:"corePath"`
	IsDefault bool   `json:"isDefault"`
}

// CoreValidateResult 内核路径验证结果
type CoreValidateResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// CoreExtendedInfo 内核扩展信息
type CoreExtendedInfo struct {
	CoreId        string `json:"coreId"`
	ChromeVersion string `json:"chromeVersion"`
	InstanceCount int    `json:"instanceCount"`
}

// Group 实例分组
type Group struct {
	GroupId   string `json:"groupId"`
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"` // 空字符串表示根级分组
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// GroupInput 创建/更新分组的输入
type GroupInput struct {
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
}

// GroupWithCount 带实例计数的分组
type GroupWithCount struct {
	Group
	InstanceCount int `json:"instanceCount"`
}

// 类型别名
type Proxy = config.BrowserProxy
type Core = config.BrowserCore
type Environment = config.BrowserEnvironment
type ProfileConfig = config.BrowserProfileConfig

// ProxySource 代理订阅源（独立建模，与代理行解耦）
type ProxySource struct {
	SourceID         string `json:"sourceId"`
	SourceURL        string `json:"sourceUrl"`
	SourceName       string `json:"sourceName"`
	GroupName        string `json:"groupName"`
	NamePrefix       string `json:"namePrefix"`
	DnsServers       string `json:"dnsServers"`
	AutoRefresh      bool   `json:"autoRefresh"`
	RefreshIntervalM int    `json:"refreshIntervalM"`
	ImportStrategy   string `json:"importStrategy"` // merge | replace
	LastRefreshAt    string `json:"lastRefreshAt"`
	LastRefreshError string `json:"lastRefreshError"`
	CreatedAt        string `json:"createdAt"`
}

// ProxySourceOverride 订阅源节点的用户覆盖记录（忽略 / 重命名）
type ProxySourceOverride struct {
	SourceID   string `json:"sourceId"`
	NodeKey    string `json:"nodeKey"`
	Action     string `json:"action"` // ignore | rename
	CustomName string `json:"customName"`
}

// CodeProvider 提供 LaunchCode 的接口（由 launchcode.LaunchCodeService 实现）
type CodeProvider interface {
	EnsureCode(profileId string) (string, error)
	Remove(profileId string) error
}

// Manager 浏览器管理器
type Manager struct {
	Config           *config.Config
	AppRoot          string // 应用根目录，所有相对路径基于此解析（生产=exe目录，dev=项目根目录）
	Profiles         map[string]*Profile
	Mutex            sync.Mutex
	BrowserProcesses map[string]*exec.Cmd
	XrayBridges      map[string]*XrayBridge
	CodeProvider     CodeProvider
	// StartingProfiles 记录正在启动中的实例，用于防止同一实例并发重复启动。
	// 由 Mutex 保护。键为 profileId。
	StartingProfiles map[string]bool

	// DAO 层（注入后使用 SQLite 存储，未注入时降级到 config.yaml）
	ProfileDAO     ProfileDAO
	ProxyDAO       ProxyDAO
	ProxySourceDAO ProxySourceDAO
	CoreDAO        CoreDAO
	BookmarkDAO    BookmarkDAO
	GroupDAO       GroupDAO
}

// XrayBridge Xray 桥接进程
type XrayBridge struct {
	NodeKey   string
	Port      int
	Cmd       *exec.Cmd
	Pid       int
	Running   bool
	LastError string
}

// NewManager 创建浏览器管理器
func NewManager(cfg *config.Config, appRoot string) *Manager {
	return &Manager{
		Config:           cfg,
		AppRoot:          appRoot,
		Profiles:         make(map[string]*Profile),
		BrowserProcesses: make(map[string]*exec.Cmd),
		XrayBridges:      make(map[string]*XrayBridge),
		StartingProfiles: make(map[string]bool),
	}
}

// ResolveRelativePath 将相对路径解析为绝对路径（基于 AppRoot）。
// 如果传入的已经是绝对路径则直接返回。
func (m *Manager) ResolveRelativePath(p string) string {
	return apppath.Resolve(m.AppRoot, p)
}
