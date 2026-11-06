<#
.SYNOPSIS
  Compile et lance pairing_easy depuis les sources, sous Windows.

.DESCRIPTION
  Modes :
    run   (défaut) compile l'interface et le serveur, puis lance dist\pairing_easy.exe
    dev   serveur Go + interface Vite avec rechargement à chaud (http://localhost:5173)
    build compile seulement dist\pairing_easy.exe
    test  lance les tests Go

  Les paramètres après le mode sont passés à pairing_easy (ex. -lan, -port 8081).

.EXAMPLE
  .\build\dev-windows.ps1
  .\build\dev-windows.ps1 dev
  .\build\dev-windows.ps1 run -lan -port 8081
#>
param(
  [ValidateSet("run", "dev", "build", "test")]
  [string]$Mode = "run",
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$AppArgs
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

function Fail($msg) {
  Write-Host ""
  Write-Host "ERREUR : $msg" -ForegroundColor Red
  Write-Host ""
  exit 1
}

function Step($msg) { Write-Host "==> $msg" -ForegroundColor Green }

# ------------------------------------------------------------ prérequis
function Require-Go {
  $go = Get-Command go -ErrorAction SilentlyContinue
  if (-not $go) {
    Fail "Go n'est pas installé. Installez-le (1.27 ou plus) : winget install GoLang.Go  -- puis rouvrez cette fenêtre."
  }
  $v = (& go env GOVERSION) -replace "^go", ""
  $parts = $v.Split(".")
  if ([int]$parts[0] -lt 1 -or ([int]$parts[0] -eq 1 -and [int]$parts[1] -lt 27)) {
    Write-Host "Go $v détecté : la version 1.27 sera téléchargée automatiquement (GOTOOLCHAIN=auto)." -ForegroundColor Yellow
    $env:GOTOOLCHAIN = "auto"
  }
}

function Require-Node {
  $node = Get-Command node -ErrorAction SilentlyContinue
  if (-not $node) {
    Fail "Node.js n'est pas installé. Installez-le (20 ou plus) : winget install OpenJS.NodeJS.LTS  -- puis rouvrez cette fenêtre."
  }
  $major = [int]((& node --version) -replace "^v", "").Split(".")[0]
  if ($major -lt 20) { Fail "Node.js $(& node --version) est trop ancien : il faut la version 20 ou plus." }
}

# ------------------------------------------------------------ étapes
function Build-Front {
  Step "Interface (npm)"
  Push-Location frontend
  try {
    if (-not (Test-Path node_modules)) { npm ci --no-audit --no-fund; if ($LASTEXITCODE) { Fail "npm ci a échoué" } }
    npm run build; if ($LASTEXITCODE) { Fail "la compilation de l'interface a échoué" }
  } finally { Pop-Location }
  Remove-Item -Recurse -Force backend\internal\web\dist -ErrorAction SilentlyContinue
  Copy-Item -Recurse frontend\dist backend\internal\web\dist
}

function Build-Back {
  Step "Serveur (go build)"
  $version = "dev-" + (Get-Date -Format "yyyy.MM.dd.HHmm")
  New-Item -ItemType Directory -Force dist | Out-Null
  Push-Location backend
  try {
    $env:CGO_ENABLED = "0"
    go build -mod=vendor -trimpath -ldflags "-s -w -X pairingeasy/internal/api.Version=$version" -o ..\dist\pairing_easy.exe .\cmd\pairing_easy
    if ($LASTEXITCODE) { Fail "la compilation du serveur a échoué" }
  } finally { Pop-Location }
  Write-Host "    dist\pairing_easy.exe ($version)"
}

# ------------------------------------------------------------ modes
switch ($Mode) {
  "build" {
    Require-Go; Require-Node
    Build-Front; Build-Back
    Step "Fait : dist\pairing_easy.exe"
  }
  "run" {
    Require-Go; Require-Node
    Build-Front; Build-Back
    Step "Lancement (fermez cette fenêtre ou Ctrl+C pour arrêter)"
    & .\dist\pairing_easy.exe @AppArgs
  }
  "dev" {
    Require-Go; Require-Node
    Push-Location frontend
    try { if (-not (Test-Path node_modules)) { npm ci --no-audit --no-fund } } finally { Pop-Location }
    # Données séparées : le développement ne touche pas la vraie base.
    $data = Join-Path $Root ".data-dev"
    New-Item -ItemType Directory -Force $data | Out-Null
    if (-not (Test-Path backend\internal\web\dist\index.html)) {
      New-Item -ItemType Directory -Force backend\internal\web\dist | Out-Null
      Set-Content backend\internal\web\dist\index.html "<!doctype html><title>pairing_easy</title><p>Interface servie par Vite sur http://localhost:5173</p>"
    }
    Step "Serveur Go sur http://localhost:8080 (données : .data-dev)"
    $backArgs = @("run", "-mod=vendor", ".\cmd\pairing_easy", "-no-browser", "-data", $data) + $AppArgs
    Start-Process -FilePath "go" -ArgumentList $backArgs -WorkingDirectory (Join-Path $Root "backend")
    Step "Interface Vite sur http://localhost:5173 (rechargement à chaud)"
    Start-Sleep -Seconds 2
    Start-Process "http://localhost:5173"
    Push-Location frontend
    try { npm run dev } finally { Pop-Location }
  }
  "test" {
    Require-Go
    Step "Tests Go"
    Push-Location backend
    try { go test -mod=vendor ./...; if ($LASTEXITCODE) { Fail "des tests échouent" } } finally { Pop-Location }
  }
}
