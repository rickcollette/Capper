package cli

import "testing"

func TestParseResourceFlags(t *testing.T) {
	resources, err := parseResourceFlags("128M", 60, 64, "1.5M", true, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if resources.Limits.MemoryBytes != 128*1024*1024 {
		t.Fatalf("unexpected memory bytes: %d", resources.Limits.MemoryBytes)
	}
	if resources.Limits.CPUTimeSecs != 60 {
		t.Fatalf("unexpected cpu time: %d", resources.Limits.CPUTimeSecs)
	}
	if resources.Limits.MaxProcesses != 64 {
		t.Fatalf("unexpected pids: %d", resources.Limits.MaxProcesses)
	}
	if resources.Limits.FileSizeBytes != int64(1.5*1024*1024) {
		t.Fatalf("unexpected file size: %d", resources.Limits.FileSizeBytes)
	}
}

func TestParseResourceFlagsRejectsInvalidValues(t *testing.T) {
	if _, err := parseResourceFlags("-1M", 0, 0, "", true, false, false, false); err == nil {
		t.Fatal("expected invalid memory error")
	}
	if _, err := parseResourceFlags("", -1, 0, "", false, true, false, false); err == nil {
		t.Fatal("expected invalid cpu time error")
	}
	if _, err := parseResourceFlags("", 0, -1, "", false, false, true, false); err == nil {
		t.Fatal("expected invalid pids error")
	}
}

func TestParseResourceFlagsOnlySetsChangedFields(t *testing.T) {
	resources, err := parseResourceFlags("", 30, 0, "", false, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resources.CPUTimeSet || resources.MemorySet || resources.PidsSet || resources.FileSizeSet {
		t.Fatalf("unexpected changed fields: %#v", resources)
	}
}

func TestParseMountSpecs(t *testing.T) {
	mounts, err := parseMountSpecs([]string{"/src:/dst", "/data:/data:ro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(mounts))
	}
	if mounts[0].Source != "/src" || mounts[0].Target != "/dst" || mounts[0].ReadOnly {
		t.Errorf("unexpected first mount: %+v", mounts[0])
	}
	if !mounts[1].ReadOnly {
		t.Error("expected second mount to be read-only")
	}
}

func TestParseMountSpecsInvalid(t *testing.T) {
	if _, err := parseMountSpecs([]string{"nocolon"}); err == nil {
		t.Error("expected error for missing colon")
	}
	if _, err := parseMountSpecs([]string{"/a:/b:badopt"}); err == nil {
		t.Error("expected error for bad option")
	}
}

func TestParsePublishSpecs(t *testing.T) {
	ports, err := parsePublishSpecs([]string{"8080:80", "5432:5432/tcp", "9000:9000/udp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
	if ports[0].HostPort != 8080 || ports[0].ContainerPort != 80 || ports[0].Protocol != "tcp" {
		t.Errorf("unexpected first port: %+v", ports[0])
	}
	if ports[2].Protocol != "udp" {
		t.Errorf("expected udp, got %s", ports[2].Protocol)
	}
}

func TestParsePublishSpecsInvalid(t *testing.T) {
	if _, err := parsePublishSpecs([]string{"notaport:80"}); err == nil {
		t.Error("expected error for non-numeric host port")
	}
	if _, err := parsePublishSpecs([]string{"8080:80/sctp"}); err == nil {
		t.Error("expected error for unknown protocol")
	}
}
