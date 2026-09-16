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
	redirectToHTTP := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset := strings.TrimPrefix(r.URL.Path, "/v1.2.3/")
		if redirectToHTTP && strings.HasPrefix(asset, "kvantumci-windows-") {
			http.Redirect(w, r, "http://example.invalid/downgrade", http.StatusFound)
			return
		}
		content, ok := files[asset]
		if !ok || !strings.HasPrefix(r.URL.Path, "/v1.2.3/") {
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
	if err := os.WriteFile(installer, []byte(strings.ReplaceAll(string(installerSource), "PROVISION_PRODUCTION_CERT_SHA256", fmt.Sprintf("%x", certHash))), 0600); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(tmp, "run.ps1")
	if err := os.WriteFile(runner, []byte("$ErrorActionPreference='Stop'\n[Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }\n& $env:KVANTUMCI_INSTALLER_SCRIPT -NoPathUpdate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KVANTUMCI_INSTALLER_SCRIPT", installer)
	t.Setenv("KVANTUMCI_PUBLIC_KEY_FILE", certPath)
	t.Setenv("KVANTUMCI_DOWNLOAD_BASE_URL", server.URL)
	t.Setenv("KVANTUMCI_INSTALL_DIR", installDir)
	t.Setenv("KVANTUMCI_VERSION", "v1.2.3")
	run := func(wantSuccess bool) {
		t.Helper()
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", runner)
		output, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("installer success=%t, wanted %t: %v\n%s", err == nil, wantSuccess, err, output)
		}
	}
	assertOld := func() {
		t.Helper()
		data, err := os.ReadFile(dest)
		if err != nil || string(data) != "old binary" {
			t.Fatalf("existing binary changed on failure: %q, %v", data, err)
		}
	}
	run(true)
	asset := "kvantumci-windows-" + runtime.GOARCH + ".exe"
	data, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(data, files[asset]) {
		t.Fatalf("installed binary mismatch: %q, %v", data, err)
	}
	pathRunner := filepath.Join(tmp, "path-test.ps1")
	pathScript := `$ErrorActionPreference='Stop'
[Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
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
	pathCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", pathRunner)
	if output, err := pathCmd.CombinedOutput(); err != nil {
		t.Fatalf("Windows PATH checks failed: %v\n%s", err, output)
	}
	reset := func() {
		t.Helper()
		if err := os.WriteFile(dest, []byte("old binary"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	reset()
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
	redirectToHTTP = true
	run(false)
	assertOld()
}
