//go:build windows

package config_test

import (
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/hackihub/kvantumcli/internal/config"
	"golang.org/x/sys/windows"
)

func TestSaveWindowsUsesProtectedCurrentUserACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("KVANTUMCI_CONFIG", path)
	if _, err := config.Save(config.Config{Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Save(config.Config{Token: "replacement-token"}); err != nil {
		t.Fatalf("replace existing Windows config: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("config ACL inherits parent permissions: %s", sd)
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl == nil || dacl.AceCount != 2 {
		t.Fatalf("expected exactly two private config ACEs: %s", sd)
	}
	var foundUser, foundSystem bool
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		// FILE_ALL_ACCESS: standard rights, synchronize, and all file-specific rights.
		const fileAllAccess = 0x001f01ff
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags != 0 || ace.Mask != fileAllAccess {
			t.Fatalf("unexpected config ACE type, flags, or permissions: %+v", ace)
		}
		// Compare binary SIDs: Windows may format the current user as an SDDL
		// alias (for example LA) instead of its numeric SID string.
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		switch {
		case sid.Equals(system):
			foundSystem = true
		case sid.Equals(user.User.Sid):
			foundUser = true
		default:
			t.Fatalf("unexpected config trustee: %s", sid)
		}
	}
	if !foundUser || !foundSystem {
		t.Fatalf("config ACL missing user or SYSTEM: %s", sd)
	}
}
