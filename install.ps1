[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string] $Version = $(if ($env:KVANTUMCI_VERSION) { $env:KVANTUMCI_VERSION } else { "latest" }),

    [string] $InstallDir = $(if ($env:KVANTUMCI_INSTALL_DIR) { $env:KVANTUMCI_INSTALL_DIR } else { Join-Path ([Environment]::GetFolderPath("LocalApplicationData")) "Programs\KvantumCI" }),

    [switch] $NoPathUpdate
)

$ErrorActionPreference = "Stop"
$repository = if ($env:KVANTUMCI_REPOSITORY) { $env:KVANTUMCI_REPOSITORY } else { "HackiHub/kvantumcli" }

if ($Version -ne "latest" -and $Version -notmatch '^v?[A-Za-z0-9][A-Za-z0-9._+-]*$') {
    throw "Invalid version: $Version"
}
if ($repository -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
    throw "Invalid GitHub repository: $repository"
}
if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
    throw "This installer only supports Windows"
}

$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$arch = switch ($architecture) {
    "x64" { "amd64" }
    "arm64" { "arm64" }
    default { throw "Unsupported Windows architecture: $architecture" }
}

$asset = "kvantumci-windows-$arch.exe"
if ($env:KVANTUMCI_DOWNLOAD_BASE_URL) {
    $url = "$($env:KVANTUMCI_DOWNLOAD_BASE_URL.TrimEnd('/'))/$asset"
} elseif ($Version -eq "latest") {
    $url = "https://github.com/$repository/releases/latest/download/$asset"
} else {
    $tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
    $url = "https://github.com/$repository/releases/download/$tag/$asset"
}

$temporaryFile = Join-Path ([IO.Path]::GetTempPath()) ("kvantumci-" + [guid]::NewGuid().ToString("N") + ".exe")
try {
    Write-Host "Downloading kvantumci $Version for windows/$arch..."
    Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $temporaryFile
    if ((Get-Item $temporaryFile).Length -eq 0) {
        throw "Downloaded file is empty"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destination = Join-Path $InstallDir "kvantumci.exe"
    Move-Item -Path $temporaryFile -Destination $destination -Force
} finally {
    if (Test-Path $temporaryFile) {
        Remove-Item $temporaryFile -Force
    }
}

if (-not $NoPathUpdate) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @($userPath -split ';' | Where-Object { $_ })
    if ($pathEntries -notcontains $InstallDir) {
        $newUserPath = (@($pathEntries) + $InstallDir) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "Added $InstallDir to your user PATH (new terminals will pick it up)."
    }
}

Write-Host "Installed kvantumci to $destination"
