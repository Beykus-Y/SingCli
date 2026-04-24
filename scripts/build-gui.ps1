$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$Wails = Get-Command wails -ErrorAction SilentlyContinue
if (-not $Wails) {
    $GoBin = Join-Path (go env GOPATH) "bin\wails.exe"
    if (-not (Test-Path $GoBin)) {
        Write-Host "Installing Wails CLI v2.12.0..."
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
    }
    $WailsPath = $GoBin
} else {
    $WailsPath = $Wails.Source
}

$HostArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$GoArch = if ($HostArch -eq "AMD64") { "amd64" } elseif ($HostArch -eq "ARM64") { "arm64" } else { go env GOARCH }
$BuildArgs = @("build", "-clean", "-platform", "windows/$GoArch", "-tags", "with_quic,with_utls,with_xtls,with_gvisor")
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Host "npm was not found in PATH; using existing cmd\myvpn-gui\frontend\dist assets."
    $BuildArgs += "-s"
}

Push-Location "cmd\myvpn-gui"
try {
    & $WailsPath @BuildArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

New-Item -ItemType Directory -Force -Path "bin" | Out-Null
if (-not (Test-Path "build\bin\mgb-gui.exe")) {
    throw "Wails build did not create build\bin\mgb-gui.exe"
}
Copy-Item -Force "build\bin\mgb-gui.exe" "bin\mgb-gui.exe"

$WintunArch = if ($GoArch -eq "386") { "x86" } else { $GoArch }
$SingTunDir = go list -m -f "{{.Dir}}" github.com/sagernet/sing-tun
if (Test-Path "bin\wintun.dll") {
    Set-ItemProperty -Path "bin\wintun.dll" -Name IsReadOnly -Value $false
}
Copy-Item -Force (Join-Path $SingTunDir "internal\wintun\$WintunArch\wintun.dll") "bin\wintun.dll"
Copy-Item -Force "bin\wintun.dll" "build\bin\wintun.dll"

Write-Host "Built bin\mgb-gui.exe with QUIC, uTLS, XTLS and gVisor support."
Write-Host "Copied bin\wintun.dll for $GoArch."
