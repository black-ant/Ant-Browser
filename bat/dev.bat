@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."
set "EXIT_CODE=0"
set "NO_PAUSE=0"
set "SHOW_USAGE=0"
set "DEV_MODE=stable"
set "DEFAULT_WORKSPACE_INSTALL_ROOT=%USERPROFILE%\Codex\1688shopManager\desktop-repos\1688shop-desktop"
set "DEFAULT_WORKSPACE_SERVER_ROOT=%USERPROFILE%\Codex\1688shopManager"
set "DEFAULT_WORKSPACE_SERVER_URL=http://127.0.0.1:4174/api/health"
set "INSTALL_ROOT_ARG="
set "INSTALL_ROOT="
set "AGENT_ENTRY="
set "WORKSPACE_SERVER_ROOT_ENV=%WORKSPACE_SERVER_ROOT%"
set "LIMITED_WATCHER_PID_FILE=tmp-frontend-limited-watcher.pid"
set "PREFERRED_FRONTEND_PORT=5218"
set "FRONTEND_PORT="
set "WATCHER_PID="
set "WATCHER_STARTED=0"
set "WORKSPACE_SERVER_ROOT="
set "WORKSPACE_SERVER_OUT_LOG="
set "WORKSPACE_SERVER_ERR_LOG="
set "WORKSPACE_SERVER_PID="
set "WORKSPACE_SERVER_STARTED=0"

call :parse_args %*
if errorlevel 1 (
    set "EXIT_CODE=1"
    goto :finish
)

if "%SHOW_USAGE%"=="1" (
    call :print_usage
    goto :finish
)

if /I "%DEV_MODE%"=="stable" (
    call :run_stable
    set "EXIT_CODE=%errorlevel%"
    goto :finish
)

if /I "%DEV_MODE%"=="live" (
    call :run_live 0
    set "EXIT_CODE=%errorlevel%"
    goto :finish
)

if /I "%DEV_MODE%"=="limited" (
    call :run_live 1
    set "EXIT_CODE=%errorlevel%"
    goto :finish
)

echo [ERROR] Unsupported dev mode: %DEV_MODE%
set "EXIT_CODE=1"

:finish
if "%WATCHER_STARTED%"=="1" call :cleanup_watcher >nul 2>&1
if "%WORKSPACE_SERVER_STARTED%"=="1" call :cleanup_workspace_server >nul 2>&1
if "%NO_PAUSE%"=="1" exit /b %EXIT_CODE%
if "%CI%"=="1" exit /b %EXIT_CODE%

pause
exit /b %EXIT_CODE%

:parse_args
if "%~1"=="" exit /b 0
if /I "%~1"=="--no-pause" (
    set "NO_PAUSE=1"
    shift
    goto :parse_args
)
if /I "%~1"=="--help" (
    set "SHOW_USAGE=1"
    shift
    goto :parse_args
)
if /I "%~1"=="-h" (
    set "SHOW_USAGE=1"
    shift
    goto :parse_args
)
if /I "%~1"=="stable" (
    set "DEV_MODE=stable"
    shift
    goto :parse_args
)
if /I "%~1"=="live" (
    set "DEV_MODE=live"
    shift
    goto :parse_args
)
if /I "%~1"=="limited" (
    set "DEV_MODE=limited"
    shift
    goto :parse_args
)
if not defined INSTALL_ROOT_ARG (
    set "INSTALL_ROOT_ARG=%~1"
    shift
    goto :parse_args
)

echo [ERROR] Unsupported extra argument: %~1
echo.
call :print_usage
exit /b 1

:print_usage
echo Usage:
echo   bat\dev.bat [stable^|live^|limited] [workspace-install-root] [--no-pause]
echo.
echo Modes:
echo   stable   Default. Build frontend static assets and start Wails without Vite dev server.
echo   live     Start Vite watcher and connect Wails to the frontend dev server.
echo   limited  Same as live, but add Windows Job Object memory limits to the watcher chain.
echo.
echo Examples:
echo   bat\dev.bat
echo   bat\dev.bat live
echo   bat\dev.bat stable C:\Users\you\Codex\1688shopManager\desktop-repos\1688shop-desktop
echo   bat\dev.bat limited --no-pause
echo.
echo Config priority:
echo   1. ANT_BROWSER_WORKSPACE_INSTALL_ROOT
echo   2. WORKSPACE_INSTALL_ROOT
echo   3. workspace-install-root argument
echo   4. %USERPROFILE%\Codex\1688shopManager\desktop-repos\1688shop-desktop
exit /b 0

:run_stable
echo ========================================
echo   Maka Browser - Dev Launcher
echo ========================================
echo.
echo Current workdir: %CD%
echo Mode: stable
echo.

call :cleanup_dev_logs
call :apply_proxy_settings
call :resolve_workspace_install_root
if errorlevel 1 exit /b 1
call :ensure_workspace_server
if errorlevel 1 exit /b 1

echo Frontend mode: stable static assets
echo Frontend build: one-shot npm run build
echo Wails frontend dev server: disabled
call :print_proxy_settings

call :cleanup_app_processes
call bat\cleanup-frontend-dev-processes.cmd warn
if errorlevel 1 exit /b 1

call :cleanup_dev_binary
if errorlevel 1 exit /b 1

call :prepare_tooling
if errorlevel 1 exit /b 1

call :build_frontend_assets
if errorlevel 1 exit /b 1

echo Starting Wails dev...
echo Asset source: frontend\dist
echo Auto reload: disabled
echo.

wails dev -m -nogorebuild -noreload -s -skipbindings -assetdir frontend/dist
set "EXIT_CODE=%errorlevel%"

if not "%EXIT_CODE%"=="0" (
    echo.
    echo [ERROR] wails dev exited with code %EXIT_CODE%.
)

exit /b %EXIT_CODE%

:run_live
set "FRONTEND_LIMITED_MODE=%~1"

if "%FRONTEND_LIMITED_MODE%"=="1" (
    if not defined FRONTEND_PROCESS_MEMORY_LIMIT_MB set "FRONTEND_PROCESS_MEMORY_LIMIT_MB=512"
    if not defined FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB set "FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB=256"
    if not defined FRONTEND_NODE_RSS_WARN_MB set "FRONTEND_NODE_RSS_WARN_MB=256"
    if not defined FRONTEND_NODE_RSS_AUTO_RESTART set "FRONTEND_NODE_RSS_AUTO_RESTART=0"
    if not defined FRONTEND_NODE_RSS_RESTART_MAX_COUNT set "FRONTEND_NODE_RSS_RESTART_MAX_COUNT=1"
) else (
    if not defined FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB set "FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB=512"
    if not defined FRONTEND_NODE_RSS_WARN_MB set "FRONTEND_NODE_RSS_WARN_MB=384"
    if not defined FRONTEND_NODE_RSS_AUTO_RESTART set "FRONTEND_NODE_RSS_AUTO_RESTART=1"
    if not defined FRONTEND_NODE_RSS_RESTART_MAX_COUNT set "FRONTEND_NODE_RSS_RESTART_MAX_COUNT=3"
)
if not defined FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB set "FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB=16"
if not defined FRONTEND_NODE_RSS_HARD_LIMIT_MB set "FRONTEND_NODE_RSS_HARD_LIMIT_MB=0"
if not defined FRONTEND_NODE_RSS_HARD_LIMIT_HITS set "FRONTEND_NODE_RSS_HARD_LIMIT_HITS=3"
if not defined FRONTEND_NODE_RSS_RESTART_DELAY_MS set "FRONTEND_NODE_RSS_RESTART_DELAY_MS=1500"
if not defined FRONTEND_NODE_RSS_RESTART_WINDOW_MS set "FRONTEND_NODE_RSS_RESTART_WINDOW_MS=300000"
if not defined FRONTEND_NODE_MEMORY_POLL_MS set "FRONTEND_NODE_MEMORY_POLL_MS=3000"
if not defined FRONTEND_DISABLE_HMR set "FRONTEND_DISABLE_HMR=0"

echo ========================================
echo   Maka Browser - Dev Launcher
echo ========================================
echo.
echo Current workdir: %CD%
if "%FRONTEND_LIMITED_MODE%"=="1" (
    echo Mode: limited
) else (
    echo Mode: live
)
echo.

call :cleanup_dev_logs
call :apply_proxy_settings
call :resolve_workspace_install_root
if errorlevel 1 exit /b 1
call :ensure_workspace_server
if errorlevel 1 exit /b 1

if "%FRONTEND_LIMITED_MODE%"=="1" (
    echo Frontend mode: live dev server with Job Object memory limit
) else (
    echo Frontend mode: live dev server
)
echo Preferred frontend port: %PREFERRED_FRONTEND_PORT%
echo Frontend Node old-space limit: %FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB% MB
echo Frontend Node semi-space limit: %FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB% MB
echo Frontend Node RSS warning: %FRONTEND_NODE_RSS_WARN_MB% MB
echo Frontend Node RSS hard limit: %FRONTEND_NODE_RSS_HARD_LIMIT_MB% MB
echo Frontend Node RSS hard-limit hits: %FRONTEND_NODE_RSS_HARD_LIMIT_HITS%
echo Frontend Node RSS auto restart: %FRONTEND_NODE_RSS_AUTO_RESTART%
echo Frontend Node RSS restart delay: %FRONTEND_NODE_RSS_RESTART_DELAY_MS% ms
echo Frontend Node RSS restart max count: %FRONTEND_NODE_RSS_RESTART_MAX_COUNT%
echo Frontend Node RSS restart window: %FRONTEND_NODE_RSS_RESTART_WINDOW_MS% ms
echo Frontend Node RSS poll interval: %FRONTEND_NODE_MEMORY_POLL_MS% ms
echo Frontend HMR disabled: %FRONTEND_DISABLE_HMR%
if "%FRONTEND_LIMITED_MODE%"=="1" echo Frontend process memory limit: %FRONTEND_PROCESS_MEMORY_LIMIT_MB% MB
call :print_proxy_settings

call :cleanup_app_processes
call bat\cleanup-frontend-dev-processes.cmd strict
if errorlevel 1 exit /b 1

call :cleanup_dev_binary
if errorlevel 1 exit /b 1

call :resolve_frontend_dev_port
if errorlevel 1 exit /b 1

call :prepare_tooling
if errorlevel 1 exit /b 1

call :ensure_embed_dist
if errorlevel 1 exit /b 1

call :start_watcher
if errorlevel 1 exit /b 1

call :wait_for_frontend_dev_server
if errorlevel 1 exit /b 1
echo.

echo Starting Wails dev...
echo Frontend URL: http://127.0.0.1:%FRONTEND_PORT%
echo.

wails dev -m -s -skipbindings -frontenddevserverurl http://127.0.0.1:%FRONTEND_PORT% -viteservertimeout 60
set "EXIT_CODE=%errorlevel%"

if not "%EXIT_CODE%"=="0" (
    echo.
    echo [ERROR] wails dev exited with code %EXIT_CODE%.
)

exit /b %EXIT_CODE%

:apply_proxy_settings
if defined DEV_PROXY_URL (
    set "HTTP_PROXY=%DEV_PROXY_URL%"
    set "HTTPS_PROXY=%DEV_PROXY_URL%"
    set "http_proxy=%DEV_PROXY_URL%"
    set "https_proxy=%DEV_PROXY_URL%"
)
if defined DEV_NO_PROXY (
    set "NO_PROXY=%DEV_NO_PROXY%"
    set "no_proxy=%DEV_NO_PROXY%"
)
if defined DEV_GOPROXY set "GOPROXY=%DEV_GOPROXY%"
if not defined DEV_GOPROXY if not defined GOPROXY set "GOPROXY=https://goproxy.cn,direct"
if not defined ANT_BROWSER_DEBUG_STARTUP set "ANT_BROWSER_DEBUG_STARTUP=1"
exit /b 0

:print_proxy_settings
if defined DEV_PROXY_URL (
    echo HTTP/HTTPS proxy: %DEV_PROXY_URL%
) else (
    echo HTTP/HTTPS proxy: disabled
)
if defined DEV_NO_PROXY (
    echo NO_PROXY: %DEV_NO_PROXY%
)
echo Go proxy: %GOPROXY%
echo.
exit /b 0

:resolve_workspace_install_root
set "INSTALL_ROOT="
if defined ANT_BROWSER_WORKSPACE_INSTALL_ROOT (
    set "INSTALL_ROOT=%ANT_BROWSER_WORKSPACE_INSTALL_ROOT%"
)
if not defined INSTALL_ROOT if defined WORKSPACE_INSTALL_ROOT (
    set "INSTALL_ROOT=%WORKSPACE_INSTALL_ROOT%"
)
if not defined INSTALL_ROOT if defined INSTALL_ROOT_ARG (
    set "INSTALL_ROOT=%INSTALL_ROOT_ARG%"
)
if not defined INSTALL_ROOT if exist "%DEFAULT_WORKSPACE_INSTALL_ROOT%\apps\agent\src\server\index.mjs" (
    set "INSTALL_ROOT=%DEFAULT_WORKSPACE_INSTALL_ROOT%"
)
if not defined INSTALL_ROOT (
    echo [ERROR] Workspace install root is required.
    echo         Set ANT_BROWSER_WORKSPACE_INSTALL_ROOT, WORKSPACE_INSTALL_ROOT,
    echo         or pass [workspace-install-root] to bat\dev.bat.
    exit /b 1
)
set "AGENT_ENTRY=%INSTALL_ROOT%\apps\agent\src\server\index.mjs"
if not exist "%AGENT_ENTRY%" (
    echo [ERROR] Invalid workspace install root: %INSTALL_ROOT%
    echo         Missing: %AGENT_ENTRY%
    exit /b 1
)
set "ANT_BROWSER_WORKSPACE_INSTALL_ROOT=%INSTALL_ROOT%"
echo Workspace install root: %ANT_BROWSER_WORKSPACE_INSTALL_ROOT%
exit /b 0

:ensure_workspace_server
call :check_workspace_server_health
if not errorlevel 1 (
    echo Workspace server: already healthy on %DEFAULT_WORKSPACE_SERVER_URL%
    echo.
    exit /b 0
)

call :resolve_workspace_server_root
if errorlevel 1 (
    echo [ERROR] Workspace server is not reachable at %DEFAULT_WORKSPACE_SERVER_URL%
    echo         and no local server root was found.
    echo         Set ANT_BROWSER_WORKSPACE_SERVER_ROOT or start npm run server manually.
    exit /b 1
)

echo Workspace server: starting from %WORKSPACE_SERVER_ROOT%
set "WORKSPACE_SERVER_OUT_LOG=%CD%\tmp-workspace-server.log"
set "WORKSPACE_SERVER_ERR_LOG=%CD%\tmp-workspace-server.err.log"
for /f "usebackq delims=" %%a in (`powershell -NoProfile -ExecutionPolicy Bypass -File "bat\start-workspace-server.ps1" -WorkspaceServerRoot "%WORKSPACE_SERVER_ROOT%" -StdoutLog "%WORKSPACE_SERVER_OUT_LOG%" -StderrLog "%WORKSPACE_SERVER_ERR_LOG%"`) do (
    if not defined WORKSPACE_SERVER_PID set "WORKSPACE_SERVER_PID=%%a"
)
if not defined WORKSPACE_SERVER_PID (
    echo [ERROR] Failed to start workspace server.
    exit /b 1
)
set "WORKSPACE_SERVER_STARTED=1"
call :wait_for_workspace_server_health
if errorlevel 1 (
    echo [ERROR] Workspace server did not become ready: %DEFAULT_WORKSPACE_SERVER_URL%
    if exist "tmp-workspace-server.err.log" type "tmp-workspace-server.err.log"
    if exist "tmp-workspace-server.log" type "tmp-workspace-server.log"
    exit /b 1
)
echo [OK] Workspace server PID: %WORKSPACE_SERVER_PID%
echo.
exit /b 0

:check_workspace_server_health
powershell -NoProfile -Command "try { $ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -Uri '%DEFAULT_WORKSPACE_SERVER_URL%' -TimeoutSec 2 | Out-Null; exit 0 } catch { exit 1 }"
exit /b %errorlevel%

:wait_for_workspace_server_health
powershell -NoProfile -Command "$deadline=(Get-Date).AddSeconds(30); while((Get-Date) -lt $deadline){ try { $ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -Uri '%DEFAULT_WORKSPACE_SERVER_URL%' -TimeoutSec 2 | Out-Null; exit 0 } catch { Start-Sleep -Milliseconds 500 } }; exit 1"
exit /b %errorlevel%

:resolve_workspace_server_root
set "WORKSPACE_SERVER_ROOT="
call :use_workspace_server_root "%ANT_BROWSER_WORKSPACE_SERVER_ROOT%"
if not errorlevel 1 goto :workspace_server_root_ready
call :use_workspace_server_root "%WORKSPACE_SERVER_ROOT_ENV%"
if not errorlevel 1 goto :workspace_server_root_ready
if not defined WORKSPACE_SERVER_ROOT for %%I in ("%ANT_BROWSER_WORKSPACE_INSTALL_ROOT%\..\..") do call :use_workspace_server_root "%%~fI"
if defined WORKSPACE_SERVER_ROOT goto :workspace_server_root_ready
call :use_workspace_server_root "%DEFAULT_WORKSPACE_SERVER_ROOT%"
if errorlevel 1 exit /b 1

:workspace_server_root_ready
set "ANT_BROWSER_WORKSPACE_SERVER_ROOT=%WORKSPACE_SERVER_ROOT%"
echo Workspace server root: %ANT_BROWSER_WORKSPACE_SERVER_ROOT%
exit /b 0

:use_workspace_server_root
set "WORKSPACE_SERVER_ROOT_CANDIDATE=%~1"
if not defined WORKSPACE_SERVER_ROOT_CANDIDATE exit /b 1
if not exist "%WORKSPACE_SERVER_ROOT_CANDIDATE%\server\index.mjs" exit /b 1
if not exist "%WORKSPACE_SERVER_ROOT_CANDIDATE%\package.json" exit /b 1
set "WORKSPACE_SERVER_ROOT=%WORKSPACE_SERVER_ROOT_CANDIDATE%"
exit /b 0

:cleanup_app_processes
echo Cleaning stale app processes...
taskkill /F /IM ant-chrome-dev.exe >nul 2>&1
echo.
exit /b 0

:cleanup_dev_binary
echo Removing stale dev binary...
if exist "build\bin\ant-chrome-dev.exe" (
    powershell -NoProfile -Command "$p='build\\bin\\ant-chrome-dev.exe'; for($i=0;$i -lt 5;$i++){ if(-not (Test-Path $p)){ exit 0 }; Remove-Item -Path $p -Force -ErrorAction SilentlyContinue; Start-Sleep -Seconds 1 }; if(Test-Path $p){ exit 2 } else { exit 0 }"
    if errorlevel 1 (
        echo [ERROR] Cannot remove build\bin\ant-chrome-dev.exe.
        echo         End ant-chrome-dev.exe in Task Manager and retry.
        exit /b 1
    )
)
if exist "build\bin\ant-chrome-dev.exe~" del /F /Q "build\bin\ant-chrome-dev.exe~" >nul 2>&1
echo.
exit /b 0

:resolve_frontend_dev_port
echo Resolving frontend dev port...
set "FRONTEND_PORT="
for /f "usebackq delims=" %%a in (`node frontend\scripts\dev-port-helper.mjs resolve --preferred %PREFERRED_FRONTEND_PORT%`) do (
    if not defined FRONTEND_PORT set "FRONTEND_PORT=%%a"
)
if not defined FRONTEND_PORT (
    echo [ERROR] Failed to resolve frontend dev port.
    exit /b 1
)
echo [OK] Frontend dev port: %FRONTEND_PORT%
echo.
exit /b 0

:prepare_tooling
call :check_dependencies
if errorlevel 1 exit /b 1

call :download_go_dependencies
if errorlevel 1 exit /b 1

call :install_frontend_dependencies
if errorlevel 1 exit /b 1

call :regenerate_bindings
if errorlevel 1 exit /b 1

exit /b 0

:check_dependencies
echo Checking dependencies...
if not exist "go.mod" (
    echo [ERROR] go.mod not found in repository root.
    exit /b 1
)
if not exist "wails.json" (
    echo [ERROR] wails.json not found in repository root.
    exit /b 1
)
exit /b 0

:download_go_dependencies
echo Downloading Go dependencies...
go mod download
if errorlevel 1 (
    echo [ERROR] Failed to download Go dependencies.
    exit /b 1
)
exit /b 0

:install_frontend_dependencies
if not exist "frontend\node_modules" (
    echo Installing frontend dependencies...
    pushd frontend
    call npm install
    set "NPM_INSTALL_EXIT_CODE=!errorlevel!"
    popd
    if not "!NPM_INSTALL_EXIT_CODE!"=="0" (
        echo [ERROR] Failed to install frontend dependencies.
        exit /b 1
    )
)
echo.
exit /b 0

:regenerate_bindings
echo Regenerating Wails bindings...
call bat\generate-bindings.bat --no-pause
if errorlevel 1 (
    echo [ERROR] Failed to generate Wails bindings.
    exit /b 1
)
if not exist "frontend\src\wailsjs" (
    echo [ERROR] Wails bindings output folder not found.
    exit /b 1
)
echo.
exit /b 0

:build_frontend_assets
echo Building frontend static assets...
pushd frontend
call npm run build
set "FRONTEND_BUILD_EXIT_CODE=!errorlevel!"
popd
if not "!FRONTEND_BUILD_EXIT_CODE!"=="0" (
    echo [ERROR] Frontend build failed.
    exit /b 1
)
if not exist "frontend\dist\index.html" (
    echo [ERROR] frontend\dist\index.html was not generated.
    exit /b 1
)
echo.
exit /b 0

:ensure_embed_dist
if not exist "frontend\dist" (
    mkdir "frontend\dist" >nul 2>&1
)
if not exist "frontend\dist\__wails_placeholder__.txt" (
    echo placeholder> "frontend\dist\__wails_placeholder__.txt"
)
if not exist "frontend\dist" (
    echo [ERROR] Failed to prepare frontend\dist for go:embed.
    exit /b 1
)
exit /b 0

:wait_for_frontend_dev_server
powershell -NoProfile -Command "$port=%FRONTEND_PORT%; $pid=%WATCHER_PID%; $deadline=(Get-Date).AddSeconds(20); while((Get-Date) -lt $deadline){ $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1; if($listener){ exit 0 }; if(-not (Get-Process -Id $pid -ErrorAction SilentlyContinue)){ exit 2 }; Start-Sleep -Milliseconds 500 }; exit 1"
if "%errorlevel%"=="0" (
    echo [OK] Frontend dev server is listening on %FRONTEND_PORT%.
    exit /b 0
)
if "%errorlevel%"=="2" (
    echo [ERROR] Frontend watcher exited before the dev server became ready.
) else (
    echo [ERROR] Timed out waiting for the frontend dev server on port %FRONTEND_PORT%.
)
if exist "tmp-npm-dev.err.log" type "tmp-npm-dev.err.log"
exit /b 1

:cleanup_watcher
if defined WATCHER_PID (
    taskkill /F /T /PID %WATCHER_PID% >nul 2>&1
)
if exist "%LIMITED_WATCHER_PID_FILE%" del /F /Q "%LIMITED_WATCHER_PID_FILE%" >nul 2>&1
node frontend\scripts\dev-port-helper.mjs cleanup >nul 2>&1
set "WATCHER_STARTED=0"
exit /b 0

:cleanup_workspace_server
if defined WORKSPACE_SERVER_PID (
    taskkill /F /T /PID %WORKSPACE_SERVER_PID% >nul 2>&1
)
set "WORKSPACE_SERVER_PID="
set "WORKSPACE_SERVER_STARTED=0"
exit /b 0

:start_watcher
echo Starting frontend watcher...
set "WATCHER_PID="
if "%FRONTEND_LIMITED_MODE%"=="1" (
    for /f "usebackq delims=" %%a in (`powershell -NoProfile -Command "$p = Start-Process -FilePath 'powershell.exe' -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','scripts/run-limited-frontend-dev.ps1','-WorkingDirectory','%CD%','-MemoryLimitMB','%FRONTEND_PROCESS_MEMORY_LIMIT_MB%','-MaxOldSpaceMB','%FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB%','-MaxSemiSpaceMB','%FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB%','-PidFile','%LIMITED_WATCHER_PID_FILE%' -WorkingDirectory '%CD%' -RedirectStandardOutput 'tmp-npm-dev.log' -RedirectStandardError 'tmp-npm-dev.err.log' -PassThru; Write-Output $p.Id"`) do (
        if not defined WATCHER_PID set "WATCHER_PID=%%a"
    )
) else (
    for /f "usebackq delims=" %%a in (`powershell -NoProfile -Command "$p = Start-Process -FilePath 'node' -ArgumentList 'frontend/scripts/dev-watcher.mjs' -WorkingDirectory '%CD%' -RedirectStandardOutput 'tmp-npm-dev.log' -RedirectStandardError 'tmp-npm-dev.err.log' -PassThru; Write-Output $p.Id"`) do (
        if not defined WATCHER_PID set "WATCHER_PID=%%a"
    )
)
if not defined WATCHER_PID (
    echo [ERROR] Failed to start frontend watcher.
    exit /b 1
)
set "WATCHER_STARTED=1"
echo [OK] Frontend watcher PID: %WATCHER_PID%
echo Watcher logs: tmp-npm-dev.log / tmp-npm-dev.err.log
echo.
exit /b 0

:cleanup_dev_logs
for %%f in (
    "tmp-npm-dev.err.log"
    "tmp-npm-dev.log"
    "tmp-frontend-limited-watcher.pid"
    "tmp-wails-err.log"
    "tmp-wails-out.log"
    "tmp-workspace-server.err.log"
    "tmp-workspace-server.log"
    "tmp-wails2-err.log"
    "tmp-wails2-out.log"
    "tmp-wails3-err.log"
    "tmp-wails3-out.log"
    "tmp-wails.err"
    "wails-dev-capture.log"
    "wails-dev-run.log"
    "wails-dev-stderr.log"
    "wails-dev-stdout.log"
) do (
    if exist %%~f del /F /Q %%~f >nul 2>&1
)
exit /b 0
