# Compile pairing_easy.exe sous Windows (Go >= 1.27 et Node >= 20 requis).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
$version = if ($env:VERSION) { $env:VERSION } else { Get-Date -Format "yyyy.MM.dd" }
Push-Location frontend; npm ci --no-audit --no-fund; npm run build; Pop-Location
Remove-Item -Recurse -Force backend\internal\web\dist -ErrorAction SilentlyContinue
Copy-Item -Recurse frontend\dist backend\internal\web\dist
New-Item -ItemType Directory -Force dist | Out-Null
Push-Location backend
$env:CGO_ENABLED = "0"
go build -mod=vendor -trimpath -ldflags "-s -w -X pairingeasy/internal/api.Version=$version" -o ..\dist\pairing_easy.exe .\cmd\pairing_easy
Pop-Location
Write-Host "Fait : dist\pairing_easy.exe"
