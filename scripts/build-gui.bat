@echo off
setlocal

cd /d "%~dp0\.."

if /I "%PROCESSOR_ARCHITEW6432%"=="AMD64" (
  set "GOARCH_VALUE=amd64"
) else if /I "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
  set "GOARCH_VALUE=amd64"
) else if /I "%PROCESSOR_ARCHITEW6432%"=="ARM64" (
  set "GOARCH_VALUE=arm64"
) else if /I "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
  set "GOARCH_VALUE=arm64"
) else (
  for /f "usebackq delims=" %%A in (`go env GOARCH`) do set "GOARCH_VALUE=%%A"
)
where wails >nul 2>nul
if errorlevel 1 (
  for /f "usebackq delims=" %%G in (`go env GOPATH`) do set "GOPATH_VALUE=%%G"
  if not exist "%GOPATH_VALUE%\bin\wails.exe" (
    echo Installing Wails CLI v2.12.0...
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
    if errorlevel 1 exit /b %errorlevel%
  )
  set "WAILS_CMD=%GOPATH_VALUE%\bin\wails.exe"
) else (
  set "WAILS_CMD=wails"
)

set "FRONTEND_FLAG="
where npm >nul 2>nul
if errorlevel 1 (
  echo npm was not found in PATH; using existing cmd\myvpn-gui\frontend\dist assets.
  set "FRONTEND_FLAG=-s"
)

pushd "cmd\myvpn-gui"
%WAILS_CMD% build -clean -platform "windows/%GOARCH_VALUE%" -tags "with_quic,with_utls,with_xtls,with_gvisor" %FRONTEND_FLAG%
popd
if errorlevel 1 exit /b %errorlevel%

if not exist "bin" mkdir "bin"
if not exist "build\bin\mgb-gui.exe" (
  echo Wails build did not create build\bin\mgb-gui.exe
  exit /b 1
)
copy /Y "build\bin\mgb-gui.exe" "bin\mgb-gui.exe" >nul
if errorlevel 1 exit /b %errorlevel%

if "%GOARCH_VALUE%"=="386" (
  set "WINTUN_ARCH=x86"
) else (
  set "WINTUN_ARCH=%GOARCH_VALUE%"
)

for /f "usebackq delims=" %%D in (`go list -m -f "{{.Dir}}" github.com/sagernet/sing-tun`) do set "SING_TUN_DIR=%%D"
if exist "bin\wintun.dll" attrib -R "bin\wintun.dll"
copy /Y "%SING_TUN_DIR%\internal\wintun\%WINTUN_ARCH%\wintun.dll" "bin\wintun.dll" >nul
if errorlevel 1 exit /b %errorlevel%
copy /Y "bin\wintun.dll" "build\bin\wintun.dll" >nul
if errorlevel 1 exit /b %errorlevel%

echo Built bin\mgb-gui.exe with QUIC, uTLS, XTLS and gVisor support.
echo Copied bin\wintun.dll for %GOARCH_VALUE%.
