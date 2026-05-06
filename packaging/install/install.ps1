param(
  [string]$Repo = "aporicho/lovart-reverse",
  [string]$Version = "latest",
  [string]$InstallDir = "$env:USERPROFILE\bin",
  [string]$McpClients = "auto",
  [switch]$Yes,
  [switch]$Force,
  [switch]$DryRun,
  [switch]$Json
)

$ErrorActionPreference = "Stop"

function Write-Result {
  param([bool]$Ok, [string]$Message, [string]$Asset = "", [string]$Path = "")
  if ($Json) {
    [PSCustomObject]@{
      ok = $Ok
      message = $Message
      data = @{
        repo = $Repo
        version = $Version
        asset = $Asset
        install_path = $Path
        mcp_clients = $McpClients
        dry_run = [bool]$DryRun
      }
    } | ConvertTo-Json -Compress
  } elseif ($Ok) {
    Write-Host $Message
  } else {
    Write-Error $Message
  }
}

function Fail {
  param([string]$Message)
  Write-Result -Ok:$false -Message $Message
  exit 1
}

if ([Environment]::Is64BitOperatingSystem -eq $false) {
  Fail "unsupported platform: Windows x86"
}

$Asset = "lovart-windows-x64.exe"
$InstallPath = Join-Path $InstallDir "lovart.exe"

if ($DryRun) {
  if ($Json) {
    Write-Result -Ok:$true -Message "dry run" -Asset $Asset -Path $InstallPath
  } else {
    Write-Host "Would download $Asset from $Repo ($Version)"
    Write-Host "Would install to $InstallPath"
    Write-Host "Would run: $InstallPath mcp install --clients $McpClients --yes"
  }
  exit 0
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
  Fail "gh CLI is required; install GitHub CLI and run gh auth login"
}

& gh auth status *> $null
if ($LASTEXITCODE -ne 0) {
  Fail "gh is not authenticated; run gh auth login"
}

if (-not $Yes) {
  $Answer = Read-Host "Install Lovart to $InstallPath and configure MCP clients '$McpClients'? [y/N]"
  if ($Answer -notin @("y", "Y", "yes", "YES")) {
    Fail "installation cancelled"
  }
}

$TmpDir = Join-Path ([IO.Path]::GetTempPath()) ("lovart-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
  $BinTmp = Join-Path $TmpDir "lovart.exe"
  $SumsTmp = Join-Path $TmpDir "SHA256SUMS"

  function Download-Asset {
    param([string]$Pattern, [string]$Output)
    if ($Version -eq "latest") {
      & gh release download --repo $Repo --pattern $Pattern -O $Output
    } else {
      & gh release download $Version --repo $Repo --pattern $Pattern -O $Output
    }
    if ($LASTEXITCODE -ne 0) {
      Fail "failed to download $Pattern"
    }
  }

  Download-Asset -Pattern $Asset -Output $BinTmp
  Download-Asset -Pattern "SHA256SUMS" -Output $SumsTmp

  $Line = Get-Content $SumsTmp | Where-Object { $_ -match "\s+$([regex]::Escape($Asset))$" } | Select-Object -First 1
  if (-not $Line) {
    Fail "SHA256SUMS does not contain $Asset"
  }
  $ExpectedHash = ($Line -split "\s+")[0].ToLowerInvariant()
  $ActualHash = (Get-FileHash -Algorithm SHA256 $BinTmp).Hash.ToLowerInvariant()
  if ($ExpectedHash -ne $ActualHash) {
    Fail "checksum mismatch for $Asset"
  }

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  if (Test-Path $InstallPath) {
    if (-not $Force) {
      Fail "$InstallPath already exists; rerun with -Force"
    }
    Copy-Item $InstallPath "$InstallPath.bak" -Force
  }
  Copy-Item $BinTmp $InstallPath -Force

  & $InstallPath --version *> $null
  if ($LASTEXITCODE -ne 0) {
    Fail "installed binary failed --version"
  }
  & $InstallPath self-test *> $null
  if ($LASTEXITCODE -ne 0) {
    Fail "installed binary failed self-test"
  }

  if ($McpClients -ne "none") {
    $Args = @("mcp", "install", "--clients", $McpClients, "--yes")
    if ($Force) {
      $Args += "--force"
    }
    $McpOutput = & $InstallPath @Args
    if ($LASTEXITCODE -ne 0) {
      Fail "MCP client configuration command failed"
    }
    try {
      $McpResult = $McpOutput | ConvertFrom-Json
    } catch {
      Fail "MCP client configuration returned invalid JSON: $McpOutput"
    }
    if (-not $McpResult.ok) {
      Fail "MCP client configuration failed: $McpOutput"
    }
  }

  if ($Json) {
    Write-Result -Ok:$true -Message "installed" -Asset $Asset -Path $InstallPath
  } else {
    Write-Host "Installed Lovart at $InstallPath"
    Write-Host "Run: $InstallPath --version"
  }
} finally {
  Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
