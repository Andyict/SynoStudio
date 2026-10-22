# Builds dist\StudioAlbum-x86_64-<version>.spk and distelease.json  (Windows PowerShell 5.1+)
# Usage: .uild.ps1 -Version 1.0.1 -Build 2 -Repo owner/SynoStudio
# Requires: Node.js (npm) and Go (portable Go in %USERPROFILE%\go-sdk\go is picked up automatically).
param([string]$Version = "1.0.0", [int]$Build = 1, [string]$Repo = $env:GITHUB_REPOSITORY)
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

$goBin = Join-Path $env:USERPROFILE "go-sdk\go\bin"
if (Test-Path $goBin) { $env:PATH = "$goBin;$env:PATH" }

Write-Host "==> icons"
Push-Location (Join-Path $root "backend")
go run ./tools/mkicons ..
if (-not $?) { throw "mkicons failed" }
Pop-Location

Write-Host "==> frontend"
Push-Location (Join-Path $root "frontend")
if (-not (Test-Path node_modules)) { npm ci; if (-not $?) { throw "npm ci failed" } }
npm run build
if (-not $?) { throw "frontend build failed" }
Pop-Location

Write-Host "==> backend (linux/amd64)"
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force $dist | Out-Null
Push-Location (Join-Path $root "backend")
$pkgVersion = "{0}-{1:D4}" -f $Version, $Build
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.Version=$pkgVersion -X main.UpdateRepo=$Repo" -o (Join-Path $dist "studioalbum-linux-amd64") .
$ok = $?
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
if (-not $ok) { throw "go build failed" }

Write-Host "==> spk"
$out = Join-Path $dist "StudioAlbum-x86_64-$Version.spk"
go run ./tools/mkspk -root .. -bin (Join-Path $dist "studioalbum-linux-amd64") -version $pkgVersion -out $out
if (-not $?) { throw "mkspk failed" }
Pop-Location
$md5 = (Get-FileHash -Algorithm MD5 $out).Hash.ToLower()
$size = (Get-Item $out).Length
'{"version":"' + $pkgVersion + '","md5":"' + $md5 + '","size":' + $size + '}' | Out-File -Encoding ascii (Join-Path $dist "release.json")
Write-Host "Done: $out ($pkgVersion)"
