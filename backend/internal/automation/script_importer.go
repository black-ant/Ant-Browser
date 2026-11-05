package automation

const (
	maxImportedBundleFiles = 256
	maxImportedBundleBytes = 16 << 20
)

var importManifestCandidates = []string{
	"automation.script.json",
	"ant-automation.json",
	"manifest.json",
}

type scriptImportEnvelope struct {
	Format          string               `json:"format"`
	PackageFormat   string               `json:"packageFormat"`
	ManifestVersion int                  `json:"manifestVersion"`
	Manifest        map[string]any       `json:"manifest"`
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Type            string               `json:"type"`
	Status          string               `json:"status"`
	EntryFile       string               `json:"entryFile"`
	Tags            []string             `json:"tags"`
	Selector        any                  `json:"selector"`
	SelectorText    any                  `json:"selectorText"`
	Params          any                  `json:"params"`
	ParamsText      any                  `json:"paramsText"`
	Script          string               `json:"script"`
	ScriptText      string               `json:"scriptText"`
	Notes           string               `json:"notes"`
	TargetConfig    map[string]any       `json:"targetConfig"`
	PublicAPI       map[string]any       `json:"publicAPI"`
	Source          map[string]any       `json:"source"`
	Files           []scriptTemplateFile `json:"files"`
}
