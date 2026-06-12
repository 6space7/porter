package install_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerInstallsGoBeforeBuildingFromSource(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "install.sh")
	raw, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}
	script := string(raw)

	installGo := strings.Index(script, "install_go_if_missing")
	buildBinary := strings.Index(script, "build_binary")
	if installGo == -1 {
		t.Fatal("installer must define and call install_go_if_missing")
	}
	if buildBinary == -1 {
		t.Fatal("installer must build the porter binary")
	}
	if installGo > buildBinary {
		t.Fatalf("install_go_if_missing appears after build_binary")
	}
	if !strings.Contains(script, "https://go.dev/dl/") {
		t.Fatal("installer must download Go from the official Go distribution host")
	}
}

func TestInstallerRestartsExistingPorterServiceAfterRebuild(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "install.sh")
	raw, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}
	script := string(raw)

	if !strings.Contains(script, "systemctl restart porter") {
		t.Fatal("installer must restart porter so upgrades run the newly built binary")
	}
}
