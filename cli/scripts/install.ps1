<#
.SYNOPSIS
    Install the Traceway CLI on Windows.

.DESCRIPTION
    iwr -useb https://cli.tracewayapp.com/install.ps1 | iex

.PARAMETER Version
    CLI version (X.Y.Z) to install. Falls back to $env:TRACEWAY_CLI_VERSION,
    then to the version this installer was published with.

.PARAMETER BinDir
    Install directory. Default: $env:LOCALAPPDATA\Programs\traceway.
#>
[CmdletBinding()]
param(
    [string] $Version = $(if ($env:TRACEWAY_CLI_VERSION) { $env:TRACEWAY_CLI_VERSION } else { '__TRACEWAY_CLI_TAG__' }),
    [string] $BinDir  = $(if ($env:TRACEWAY_BIN_DIR)      { $env:TRACEWAY_BIN_DIR }      else { Join-Path $env:LOCALAPPDATA 'Programs\traceway' })
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Step($msg) { Write-Host "traceway-install: $msg" }

# A pinned tag looks like 'cli/vX.Y.Z'; a user-supplied -Version is bare X.Y.Z.
$Tag = if ($Version -like 'cli/v*') { $Version } else { "cli/v$Version" }
if ([string]::IsNullOrWhiteSpace($Version) -or $Tag -eq 'cli/v__TRACEWAY_CLI_TAG__' -or $Tag -eq 'cli/v__NOT_RELEASED__') {
    throw 'this installer has not been released yet. Check https://github.com/tracewayapp/traceway/releases, then re-run with -Version X.Y.Z.'
}
$Ver = $Tag -replace '^cli/v', ''

if (-not [System.Environment]::Is64BitOperatingSystem) {
    throw 'only 64-bit Windows is supported.'
}

$Repo         = 'tracewayapp/traceway'
$Archive      = "traceway_${Ver}_windows_x86_64.zip"
$ArchiveUrl   = "https://github.com/$Repo/releases/download/$Tag/$Archive"
$ChecksumsUrl = "https://github.com/$Repo/releases/download/$Tag/checksums.txt"
$BinPath      = Join-Path $BinDir 'traceway.exe'

$Tmp = Join-Path $env:TEMP ("traceway-install-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null

try {
    Write-Step "downloading $ArchiveUrl"
    $ArchivePath = Join-Path $Tmp $Archive
    Invoke-WebRequest -UseBasicParsing -Uri $ArchiveUrl -OutFile $ArchivePath

    Write-Step 'verifying sha256'
    $ChecksumsPath = Join-Path $Tmp 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri $ChecksumsUrl -OutFile $ChecksumsPath
    $expectedLine = Get-Content $ChecksumsPath |
        Where-Object { $_ -match ("\s\*?" + [Regex]::Escape($Archive) + '$') } |
        Select-Object -First 1
    if (-not $expectedLine) { throw "no checksum entry for $Archive" }
    $expected = ($expectedLine -split '\s+')[0].ToLower()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $ArchivePath).Hash.ToLower()
    if ($expected -ne $actual) { throw "checksum mismatch for ${Archive}: expected $expected, got $actual" }

    Write-Step 'unpacking'
    Expand-Archive -Path $ArchivePath -DestinationPath $Tmp -Force
    $srcBin = Join-Path $Tmp 'traceway.exe'
    if (-not (Test-Path $srcBin)) { throw "binary not found in archive (expected traceway.exe)" }

    Write-Step "installing -> $BinPath"
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Copy-Item -Path $srcBin -Destination $BinPath -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $BinDir) {
        Write-Step "adding $BinDir to your user PATH"
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDir", 'User')
        Write-Step 'note: open a new terminal for the PATH change to take effect'
    }

    Write-Host ''
    Write-Step "installed traceway $Ver -> $BinPath"
    Write-Step 'next: traceway login --url <your-traceway-url>'
} finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}
