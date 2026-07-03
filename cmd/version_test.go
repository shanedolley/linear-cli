package cmd

import (
	"runtime"
	"testing"
)

// buildVersionInfo must report the build stamp (or "dev" locally) alongside the
// real Go toolchain and target platform, so `lincli version --json` is accurate.
func TestBuildVersionInfo_PopulatesFields(t *testing.T) {
	info := buildVersionInfo()

	if info.Version == "" {
		t.Error("Version is empty; want the build stamp or \"dev\"")
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	wantPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if info.Platform != wantPlatform {
		t.Errorf("Platform = %q, want %q", info.Platform, wantPlatform)
	}
}

// The version command needs no client or auth, so its handler must run cleanly.
func TestVersionCmd_RunsCleanly(t *testing.T) {
	if err := versionCmd.RunE(versionCmd, nil); err != nil {
		t.Fatalf("versionCmd.RunE returned error: %v", err)
	}
}
