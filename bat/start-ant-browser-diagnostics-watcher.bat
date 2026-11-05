@echo off
setlocal

set "WATCHER=%~dp0..\tools\ant-chrome-diagnostics-watcher.ps1"
set "POWERSHELL_EXE=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"

if not exist "%WATCHER%" (
    echo Diagnostics watcher not found:
    echo %WATCHER%
    timeout /t 5 /nobreak >nul
    exit /b 1
)

if not exist "%POWERSHELL_EXE%" (
    echo Windows PowerShell not found:
    echo %POWERSHELL_EXE%
    timeout /t 5 /nobreak >nul
    exit /b 1
)

echo Ant Browser diagnostics watcher is running in this window.
echo Close this window to stop the watcher.
echo Log: E:\software\Ant Browser\data\diagnostics\watcher.log
echo.
"%POWERSHELL_EXE%" -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%WATCHER%"
if errorlevel 1 (
    echo Diagnostics watcher stopped with an error.
    pause
    exit /b 1
)

echo Diagnostics watcher stopped.
pause
exit /b 0
