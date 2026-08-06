package browser

const (
	ExtensionInstallModePersistent  = "persistent"
	ExtensionInstallModeCommandline = "commandline"

	ExtensionRuntimeStatusInstalled = "installed"
	ExtensionRuntimeStatusDisabled  = "disabled"
	ExtensionRuntimeStatusError     = "error"
)

type Extension struct {
	ExtensionID  string `json:"extensionId"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	IconDataURL  string `json:"iconDataUrl"`
	ManifestJSON string `json:"manifestJson"`
	SourceURL    string `json:"sourceUrl"`
	InstallDir   string `json:"installDir"`
	InstallMode  string `json:"installMode"`
	PackagePath  string `json:"packagePath"`
	PackageHash  string `json:"packageHash"`
	Enabled      bool   `json:"enabled"`
	InstalledAt  string `json:"installedAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type ExtensionLookupResult struct {
	ExtensionID string `json:"extensionId"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	StoreURL    string `json:"storeUrl"`
	Installable bool   `json:"installable"`
	Message     string `json:"message"`
}

type ProfileExtensionSettings struct {
	ProfileID    string   `json:"profileId"`
	Configured   bool     `json:"configured"`
	ExtensionIDs []string `json:"extensionIds"`
	UpdatedAt    string   `json:"updatedAt"`
}

type ProfileExtensionRuntime struct {
	ProfileID          string `json:"profileId"`
	ExtensionID        string `json:"extensionId"`
	RuntimeExtensionID string `json:"runtimeExtensionId"`
	InstallMode        string `json:"installMode"`
	InstalledVersion   string `json:"installedVersion"`
	PackageHash        string `json:"packageHash"`
	Status             string `json:"status"`
	BackupPath         string `json:"backupPath"`
	LastVerifiedAt     string `json:"lastVerifiedAt"`
	LastError          string `json:"lastError"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}
