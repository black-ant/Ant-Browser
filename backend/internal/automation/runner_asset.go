package automation

import _ "embed"

const runnerScriptFileName = "runner.cjs"

//go:embed assets/runner.cjs
var runnerScriptContent []byte

//go:embed assets/runner_shared.cjs
var runnerSharedScriptContent []byte

//go:embed assets/runner_page_api.cjs
var runnerPageAPIScriptContent []byte

//go:embed assets/runner_script_loader.cjs
var runnerScriptLoaderContent []byte

var runnerAssetFiles = map[string][]byte{
	runnerScriptFileName:       runnerScriptContent,
	"runner_shared.cjs":        runnerSharedScriptContent,
	"runner_page_api.cjs":      runnerPageAPIScriptContent,
	"runner_script_loader.cjs": runnerScriptLoaderContent,
}
