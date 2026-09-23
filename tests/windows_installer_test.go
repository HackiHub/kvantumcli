//go:build windows

package installer_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWindowsInstaller(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("unsupported installer architecture")
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
	}, &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
	}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	certPath := filepath.Join(tmp, "test-cert.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0600); err != nil {
		t.Fatal(err)
	}
	assets := []string{"kvantumci-darwin-amd64", "kvantumci-darwin-arm64", "kvantumci-linux-amd64", "kvantumci-linux-arm64", "kvantumci-windows-amd64.exe", "kvantumci-windows-arm64.exe"}
	files := map[string][]byte{}
	var manifest bytes.Buffer
	fmt.Fprint(&manifest, "kvantumci-release-v1\nversion v1.2.3\n")
	for _, asset := range assets {
		files[asset] = []byte("new binary: " + asset + "\n")
		h := sha256.Sum256(files[asset])
		fmt.Fprintf(&manifest, "sha256 %x %s\n", h, asset)
	}
	files["release-manifest.txt"] = manifest.Bytes()
	h := sha256.Sum256(manifest.Bytes())
	files["release-manifest.sig"], err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	redirectLocation := ""
	redirectStatus := http.StatusFound
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			http.Redirect(w, r, "/tag/v1.2.3", http.StatusFound)
			return
		}
		if r.URL.Path == "/tag/v1.2.3" {
			_, _ = w.Write([]byte("release"))
			return
		}
		asset := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1.2.3/"), "/signed/")
		if redirectLocation != "" && strings.HasPrefix(r.URL.Path, "/v1.2.3/") && strings.HasPrefix(asset, "kvantumci-windows-") {
			if redirectLocation != "<missing>" {
				w.Header().Set("Location", redirectLocation)
			}
			w.WriteHeader(redirectStatus)
			return
		}
		content, ok := files[asset]
		if !ok || !(strings.HasPrefix(r.URL.Path, "/v1.2.3/") || strings.HasPrefix(r.URL.Path, "/signed/")) {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	installDir := filepath.Join(tmp, "install")
	if err := os.Mkdir(installDir, 0700); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(installDir, "kvantumci.exe")
	if err := os.WriteFile(dest, []byte("old binary"), 0700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(cwd, "..", "install.ps1")
	installerSource, err := os.ReadFile(installer)
	if err != nil {
		t.Fatal(err)
	}
	certHash := sha256.Sum256(certDER)
	installer = filepath.Join(tmp, "install-test.ps1")
	if err := os.WriteFile(installer, []byte(strings.ReplaceAll(string(installerSource), "7f200aeb7faf7e5158caa354d7a0c72bfcbca09ab024b8d557016b6a10aa197b", fmt.Sprintf("%x", certHash))), 0600); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(tmp, "run.ps1")
	runnerScript := `$ErrorActionPreference='Stop'
if ($env:KVANTUMCI_FIXTURE_CERT_HASH) {
    $expected = $env:KVANTUMCI_FIXTURE_CERT_HASH
    [Net.ServicePointManager]::ServerCertificateValidationCallback = {
        param($sender, $certificate, $chain, $errors)
        if (-not $certificate) { return $false }
        $sha = [Security.Cryptography.SHA256]::Create()
        try { $actual = [BitConverter]::ToString($sha.ComputeHash($certificate.GetRawCertData())).Replace('-', '').ToLowerInvariant() }
        finally { $sha.Dispose() }
        return $actual -ceq $expected
    }
}
& $env:KVANTUMCI_INSTALLER_SCRIPT -NoPathUpdate
`
	if err := os.WriteFile(runner, []byte(runnerScript), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_INSTALLER_SCRIPT", installer)
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", certPath)
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", server.URL)
	t.Setenv("KVANTUMCI_INSTALL_DIR", installDir)
	t.Setenv("KVANTUMCI_VERSION", "v1.2.3")
	serverCert := sha256.Sum256(server.TLS.Certificates[0].Certificate[0])
	t.Setenv("KVANTUMCI_FIXTURE_CERT_HASH", fmt.Sprintf("%x", serverCert))
	run := func(wantSuccess bool) string {
		t.Helper()
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", runner)
		output, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("installer success=%t, wanted %t: %v\n%s", err == nil, wantSuccess, err, output)
		}
		return string(output)
	}
	assertOld := func() {
		t.Helper()
		data, err := os.ReadFile(dest)
		if err != nil || string(data) != "old binary" {
			t.Fatalf("existing binary changed on failure: %q, %v", data, err)
		}
	}
	expectFailure := func(message string) {
		t.Helper()
		output := run(false)
		if !strings.Contains(output, message) {
			t.Fatalf("expected %q in installer output:\n%s", message, output)
		}
		assertOld()
	}
	run(true)
	asset := "kvantumci-windows-" + runtime.GOARCH + ".exe"
	data, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(data, files[asset]) {
		t.Fatalf("installed binary mismatch: %q, %v", data, err)
	}
	pathRunner := filepath.Join(tmp, "path-test.ps1")
	pathScript := `$ErrorActionPreference='Stop'
$fixtureCertificateHash = $env:KVANTUMCI_FIXTURE_CERT_HASH
[Net.ServicePointManager]::ServerCertificateValidationCallback = {
    param($sender, $certificate, $chain, $errors)
    if (-not $certificate) { return $false }
    $sha = [Security.Cryptography.SHA256]::Create()
    try { $actual = [BitConverter]::ToString($sha.ComputeHash($certificate.GetRawCertData())).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
    return $actual -ceq $fixtureCertificateHash
}
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
if (-not $key) { $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment') }
$original = $key.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$kind = if ($null -ne $original) { $key.GetValueKind('Path') } else { [Microsoft.Win32.RegistryValueKind]::String }
try {
    $raw = '%USERPROFILE%;%JAVA_HOME%'
    $key.SetValue('Path', $raw, [Microsoft.Win32.RegistryValueKind]::ExpandString)
    & $env:KVANTUMCI_INSTALLER_SCRIPT
    $after = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $expected = $raw + ';' + $env:KVANTUMCI_INSTALL_DIR
    if ($after -cne $expected -or $key.GetValueKind('Path') -ne [Microsoft.Win32.RegistryValueKind]::ExpandString) { throw 'Raw expandable PATH was not preserved' }
    & $env:KVANTUMCI_INSTALLER_SCRIPT
    $again = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($again -cne $expected) { throw 'Repeated install duplicated PATH' }
    $rawEquivalent = '%KVANTUMCI_FIXTURE_INSTALL_DIR%'
    $key.SetValue('Path', $rawEquivalent, [Microsoft.Win32.RegistryValueKind]::ExpandString)
    & $env:KVANTUMCI_INSTALLER_SCRIPT
    $expanded = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($expanded -cne $rawEquivalent -or $key.GetValueKind('Path') -ne [Microsoft.Win32.RegistryValueKind]::ExpandString) { throw 'Expanded equivalent PATH entry was duplicated' }
    $key.SetValue('Path', $env:KVANTUMCI_INSTALL_DIR, [Microsoft.Win32.RegistryValueKind]::String)
    & $env:KVANTUMCI_INSTALLER_SCRIPT
    $literal = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($literal -cne $env:KVANTUMCI_INSTALL_DIR) { throw 'Literal equivalent PATH entry was duplicated' }
    $key.SetValue('Path', $raw, [Microsoft.Win32.RegistryValueKind]::ExpandString)
    & $env:KVANTUMCI_INSTALLER_SCRIPT -NoPathUpdate
    $unchanged = [string] $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($unchanged -cne $raw) { throw '-NoPathUpdate changed PATH' }
} finally {
    if ($null -eq $original) { $key.DeleteValue('Path', $false) }
    else { $key.SetValue('Path', $original, $kind) }
    $key.Dispose()
}
`
	if err := os.WriteFile(pathRunner, []byte(pathScript), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_FIXTURE_INSTALL_DIR", installDir)
	pathCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", pathRunner)
	if output, err := pathCmd.CombinedOutput(); err != nil {
		t.Fatalf("Windows PATH checks failed: %v\n%s", err, output)
	}
	// Rewrite only the source URL constants in a private test copy to exercise
	// the production non-mirror and latest branches against the local HTTPS fixture.
	seamSource := string(installerSource)
	for old, replacement := range map[string]string{
		`"https://github.com/$repository/releases/latest"`:        `"` + server.URL + `/latest"`,
		`"https://github.com/$repository/releases/tag/"`:          `"` + server.URL + `/tag/"`,
		`"https://github.com/$repository/releases/download/$tag"`: `"` + server.URL + `/v1.2.3"`,
	} {
		if !strings.Contains(seamSource, old) {
			t.Fatalf("missing URL test seam: %s", old)
		}
		seamSource = strings.Replace(seamSource, old, replacement, 1)
	}
	seamSource = strings.ReplaceAll(seamSource, "7f200aeb7faf7e5158caa354d7a0c72bfcbca09ab024b8d557016b6a10aa197b", fmt.Sprintf("%x", certHash))
	seamInstaller := filepath.Join(tmp, "install-seam.ps1")
	if err := os.WriteFile(seamInstaller, []byte(seamSource), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_INSTALLER_SCRIPT", seamInstaller)
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", "")
	run(true)
	t.Setenv("KVANTUMCI_VERSION", "latest")
	run(true)
	t.Setenv("KVANTUMCI_VERSION", "v1.2.3")
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", server.URL)
	t.Setenv("KVANTUMCI_INSTALLER_SCRIPT", installer)
	reset := func() {
		t.Helper()
		if err := os.WriteFile(dest, []byte("old binary"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	reset()
	redirectLocation = server.URL + "/signed/" + asset + "?token=signed"
	run(true)
	reset()
	redirectLocation = "../../signed/" + asset + "?token=signed"
	run(true)
	reset()
	redirectLocation = "https://user:pass@example.invalid/" + asset
	expectFailure("Invalid HTTPS download URL")
	redirectLocation = "<missing>"
	expectFailure("Redirect without Location")
	redirectLocation = "/v1.2.3/" + asset
	expectFailure("Too many redirects")
	redirectLocation = ""
	wrongVersion := bytes.Replace(manifest.Bytes(), []byte("version v1.2.3"), []byte("version v1.2.2"), 1)
	files["release-manifest.txt"] = wrongVersion
	wrongHash := sha256.Sum256(wrongVersion)
	files["release-manifest.sig"], err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, wrongHash[:])
	if err != nil {
		t.Fatal(err)
	}
	expectFailure("Invalid release manifest header or version")
	files["release-manifest.txt"] = manifest.Bytes()
	files["release-manifest.sig"], err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	unpinned := filepath.Join(tmp, "unpinned.pem")
	if err := os.WriteFile(unpinned, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", unpinned)
	expectFailure("Trusted certificate fingerprint mismatch")
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", certPath)
	t.Setenv("KVANTUMCI_FIXTURE_CERT_HASH", "")
	expectFailure("Download failed")
	t.Setenv("KVANTUMCI_FIXTURE_CERT_HASH", fmt.Sprintf("%x", serverCert))
	files[asset] = []byte("tampered")
	run(false)
	assertOld()
	files[asset] = []byte("new binary: " + asset + "\n")
	files["release-manifest.txt"] = append(bytes.Clone(manifest.Bytes()), []byte("tampered\n")...)
	run(false)
	assertOld()
	files["release-manifest.txt"] = manifest.Bytes()
	files["release-manifest.sig"][0] ^= 1
	run(false)
	assertOld()
	files["release-manifest.sig"][0] ^= 1
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", "")
	run(false)
	assertOld()
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", certPath)
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", "http://example.invalid")
	run(false)
	assertOld()
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", server.URL)
	redirectLocation = "http://example.invalid/downgrade"
	expectFailure("Invalid HTTPS download URL")
}
