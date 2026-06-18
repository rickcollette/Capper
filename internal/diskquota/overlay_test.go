package diskquota

import (
	"os"
	"testing"
)

func TestSetupOverlay(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount")
	}
	inst, err := os.MkdirTemp("", "capper-overlay-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		Teardown(inst)
		_ = os.RemoveAll(inst)
	}()
	rootfs := inst + "/rootfs"
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootfs+"/marker", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetupOverlay(inst, 16<<20); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(rootfs + "/marker")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("marker = %q", b)
	}
	if err := os.WriteFile(rootfs+"/writable", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
}
