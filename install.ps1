[CmdletBinding()]
param(
    [Parameter(Position = 0)] [string] $Version = $(if ($env:KVANTUMCI_VERSION) { $env:KVANTUMCI_VERSION } else { 'latest' }),
    [string] $InstallDir = $(if ($env:KVANTUMCI_INSTALL_DIR) { $env:KVANTUMCI_INSTALL_DIR } else { Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Programs\KvantumCI' }),
    [switch] $NoPathUpdate
)
$ErrorActionPreference = 'Stop'
$repository = if ($env:KVANTUMCI_REPOSITORY) { $env:KVANTUMCI_REPOSITORY } else { 'HackiHub/kvantumcli' }
$mirror = $env:KVANTUMCI_DOWNLOAD_BASE_URL
$certificatePath = $env:KVANTUMCI_PUBLIC_KEY_FILE
$PinnedCertificateSha256 = 'PROVISION_PRODUCTION_CERT_SHA256'
function Assert-HttpsUrl([string] $Value, [bool] $AllowQuery = $false) {
    $uri = $null
    if (-not [Uri]::TryCreate($Value, [UriKind]::Absolute, [ref] $uri) -or
        $uri.Scheme -ne 'https' -or -not $uri.Host -or $uri.UserInfo -or
        (-not $AllowQuery -and $uri.Query) -or $uri.Fragment -or $Value -match '[\x00-\x20\x7f]') {
        throw "Invalid HTTPS download URL: $Value"
    }
    return $uri
}

function Receive-Https([string] $Url, [string] $Path) {
    $uri = Assert-HttpsUrl $Url
    for ($redirect = 0; $redirect -le 10; $redirect++) {
        $request = [Net.HttpWebRequest] [Net.WebRequest]::Create($uri)
        $request.AllowAutoRedirect = $false
        $request.Timeout = 30000
        try {
            $response = [Net.HttpWebResponse] $request.GetResponse()
        } catch [Net.WebException] {
            if ($_.Exception.Response) { $_.Exception.Response.Dispose() }
            throw "Download failed: $uri"
        }
        try {
            $code = [int] $response.StatusCode
            if ($code -ge 300 -and $code -lt 400) {
                $location = $response.Headers['Location']
                if (-not $location) { throw "Redirect without Location: $uri" }
                if ($location -match '[\x00-\x20\x7f]') { throw "Invalid HTTPS redirect: $uri" }
                $uri = Assert-HttpsUrl ([Uri]::new($uri, $location).AbsoluteUri) $true
                continue
            }
            if ($code -ne 200) { throw "Download failed ($code): $uri" }
            $inputStream = $response.GetResponseStream()
            $outputStream = [IO.File]::Create($Path)
            try { $inputStream.CopyTo($outputStream) } finally { $outputStream.Dispose(); $inputStream.Dispose() }
            return $uri.AbsoluteUri
        } finally {
            $response.Dispose()
        }
    }
    throw "Too many redirects: $Url"
}

if ($repository -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$' -or $repository -match '\.\.|(^|/)[.-]|[.-](/|$)') { throw 'Invalid GitHub repository' }
if ($PinnedCertificateSha256 -cnotmatch '^[0-9a-f]{64}$') { throw 'Installer trust root has not been provisioned' }
if (-not $certificatePath -or -not [IO.File]::Exists($certificatePath)) { throw 'KVANTUMCI_PUBLIC_KEY_FILE must name a trusted X.509 certificate PEM file' }
$pem = [IO.File]::ReadAllText($certificatePath)
if ($pem -notmatch '(?s)^-----BEGIN CERTIFICATE-----\s*([A-Za-z0-9+/=\s]+)\s*-----END CERTIFICATE-----\s*$') { throw 'Invalid trusted certificate PEM' }
$certificate = [Security.Cryptography.X509Certificates.X509Certificate2]::new([Convert]::FromBase64String(($Matches[1] -replace '\s', '')))
$fingerprintHasher = [Security.Cryptography.SHA256]::Create()
try { $fingerprint = [BitConverter]::ToString($fingerprintHasher.ComputeHash($certificate.RawData)).Replace('-', '').ToLowerInvariant() } finally { $fingerprintHasher.Dispose() }
if ($fingerprint -cne $PinnedCertificateSha256) { $certificate.Dispose(); throw 'Trusted certificate fingerprint mismatch' }
if (-not [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([Runtime.InteropServices.OSPlatform]::Windows)) { throw 'This installer only supports Windows' }
$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()) {
    'x64' { 'amd64' }
    'arm64' { 'arm64' }
    default { throw 'Unsupported Windows architecture' }
}
$asset = "kvantumci-windows-$arch.exe"
$tempDir = Join-Path ([IO.Path]::GetTempPath()) ("kvantumci-" + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($tempDir) | Out-Null
$staged = $null
try {
    if ($Version -eq 'latest') {
        $latestFile = Join-Path $tempDir 'latest.html'
        $effective = Receive-Https "https://github.com/$repository/releases/latest" $latestFile
        $prefix = "https://github.com/$repository/releases/tag/"
        if (-not $effective.StartsWith($prefix, [StringComparison]::Ordinal)) { throw 'Unexpected latest release URL' }
        $tag = $effective.Substring($prefix.Length)
    } else {
        $tag = if ($Version.StartsWith('v')) { $Version } else { "v$Version" }
    }
    if ($tag -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') { throw "Invalid release version: $tag" }
    $base = if ($mirror) { (Assert-HttpsUrl $mirror).AbsoluteUri.TrimEnd('/') + "/$tag" } else { "https://github.com/$repository/releases/download/$tag" }
    $manifestPath = Join-Path $tempDir 'release-manifest.txt'
    $signaturePath = Join-Path $tempDir 'release-manifest.sig'
    $binaryPath = Join-Path $tempDir $asset
    Receive-Https "$base/release-manifest.txt" $manifestPath | Out-Null
    Receive-Https "$base/release-manifest.sig" $signaturePath | Out-Null

    $manifestBytes = [IO.File]::ReadAllBytes($manifestPath)
    $signatureBytes = [IO.File]::ReadAllBytes($signaturePath)
    if ($manifestBytes.Length -eq 0 -or $signatureBytes.Length -eq 0) { throw 'Empty release manifest or signature' }
    try {
        $rsa = [Security.Cryptography.X509Certificates.RSACertificateExtensions]::GetRSAPublicKey($certificate)
        if (-not $rsa -or $rsa.KeySize -ne 3072) { throw 'Trusted certificate must contain an RSA-3072 key' }
        if (-not $rsa.VerifyData($manifestBytes, $signatureBytes, [Security.Cryptography.HashAlgorithmName]::SHA256, [Security.Cryptography.RSASignaturePadding]::Pkcs1)) { throw 'Release manifest signature is invalid' }
    } finally { if ($rsa) { $rsa.Dispose() }; $certificate.Dispose() }

    $utf8 = [Text.UTF8Encoding]::new($false, $true)
    $manifestText = $utf8.GetString($manifestBytes)
    $lines = $manifestText.Split([char] "`n")
    if ($lines.Count -ne 9 -or $lines[8] -ne '' -or $lines[0] -ne 'kvantumci-release-v1' -or $lines[1] -ne "version $tag") { throw 'Invalid release manifest header or version' }
    $assets = @('kvantumci-darwin-amd64', 'kvantumci-darwin-arm64', 'kvantumci-linux-amd64', 'kvantumci-linux-arm64', 'kvantumci-windows-amd64.exe', 'kvantumci-windows-arm64.exe')
    $expectedHash = $null
    for ($i = 0; $i -lt $assets.Count; $i++) {
        if ($lines[$i + 2] -cnotmatch '^sha256 ([0-9a-f]{64}) ([A-Za-z0-9.-]+)$' -or $Matches[2] -cne $assets[$i]) { throw 'Invalid or unsorted release manifest entry' }
        if ($assets[$i] -ceq $asset) { $expectedHash = $Matches[1] }
    }
    if (-not $expectedHash) { throw 'Asset missing from release manifest' }
    Receive-Https "$base/$asset" $binaryPath | Out-Null
    if ((Get-Item $binaryPath).Length -eq 0) { throw 'Downloaded binary is empty' }
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $stream = [IO.File]::OpenRead($binaryPath)
        try { $actualHash = [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() } finally { $stream.Dispose() }
    } finally { $sha.Dispose() }
    if ($actualHash -cne $expectedHash) { throw 'Downloaded binary hash mismatch' }

    [IO.Directory]::CreateDirectory($InstallDir) | Out-Null
    $destination = Join-Path $InstallDir 'kvantumci.exe'
    $staged = Join-Path $InstallDir ('.kvantumci-' + [guid]::NewGuid().ToString('N') + '.exe')
    [IO.File]::Copy($binaryPath, $staged)
    # PowerShell converts $null to an empty string for this .NET string parameter.
    if ([IO.File]::Exists($destination)) { [IO.File]::Replace($staged, $destination, [NullString]::Value) }
    else { [IO.File]::Move($staged, $destination) }
    $staged = $null

    if (-not $NoPathUpdate) {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
        if (-not $key) { $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment') }
        try {
            $rawPath = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
            $kind = try { $key.GetValueKind('Path') } catch { [Microsoft.Win32.RegistryValueKind]::String }
            if ($kind -ne [Microsoft.Win32.RegistryValueKind]::ExpandString) { $kind = [Microsoft.Win32.RegistryValueKind]::String }
            $entries = @($rawPath.Split(';') | Where-Object { $_ })
            $normalizedInstallDir = [Environment]::ExpandEnvironmentVariables($InstallDir).TrimEnd('\', '/')
            if (-not ($entries | Where-Object { [Environment]::ExpandEnvironmentVariables($_).TrimEnd('\', '/') -ieq $normalizedInstallDir })) {
                $newPath = (@($entries) + $InstallDir) -join ';'
                $key.SetValue('Path', $newPath, $kind)
                $env:Path = "$env:Path;$InstallDir"
                Write-Host "Added $InstallDir to your user PATH (new terminals will pick it up)."
            }
        } finally { $key.Dispose() }
    }
    Write-Host "Installed kvantumci to $destination"
} finally {
    if ($staged -and [IO.File]::Exists($staged)) { [IO.File]::Delete($staged) }
    if ([IO.Directory]::Exists($tempDir)) { [IO.Directory]::Delete($tempDir, $true) }
}
