package releasepack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackIsDeterministicAndSafe(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "profiles", "one.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "ecs-exporter"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	modTime := time.Unix(1_700_000_000, 0).UTC()
	first := filepath.Join(t.TempDir(), "first.tar.gz")
	second := filepath.Join(t.TempDir(), "second.tar.gz")
	if err := Pack(source, first, "exporter_1.0.0_linux_amd64", modTime); err != nil {
		t.Fatal(err)
	}
	if err := Pack(source, second, "exporter_1.0.0_linux_amd64", modTime); err != nil {
		t.Fatal(err)
	}
	firstData, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstData, secondData) {
		t.Fatal("archives are not deterministic")
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(firstData))
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(gzipReader)
	var names []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
		if !header.ModTime.Equal(modTime) {
			t.Fatalf("%s modtime = %s", header.Name, header.ModTime)
		}
		if header.Uid != 0 || header.Gid != 0 {
			t.Fatalf("%s ownership = %d:%d", header.Name, header.Uid, header.Gid)
		}
	}
	want := []string{
		"exporter_1.0.0_linux_amd64/",
		"exporter_1.0.0_linux_amd64/ecs-exporter",
		"exporter_1.0.0_linux_amd64/profiles/",
		"exporter_1.0.0_linux_amd64/profiles/one.json",
	}
	if !bytes.Equal([]byte(join(names)), []byte(join(want))) {
		t.Fatalf("archive names = %#v", names)
	}
}

func TestPackRejectsUnsafeInputs(t *testing.T) {
	source := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := Pack(source, filepath.Join(t.TempDir(), "x.tar.gz"), "safe", time.Now()); err == nil {
		t.Fatal("symlink was accepted")
	}
	if err := Pack(t.TempDir(), filepath.Join(t.TempDir(), "x.tar.gz"), "../unsafe", time.Now()); err == nil {
		t.Fatal("unsafe prefix was accepted")
	}
	if err := Pack(t.TempDir(), filepath.Join(t.TempDir(), "x.tar.gz"), "..", time.Now()); err == nil {
		t.Fatal("parent prefix was accepted")
	}
	directory := t.TempDir()
	sourceLink := filepath.Join(t.TempDir(), "source-link")
	if err := os.Symlink(directory, sourceLink); err != nil {
		t.Fatal(err)
	}
	if err := Pack(sourceLink, filepath.Join(t.TempDir(), "x.tar.gz"), "safe", time.Now()); err == nil {
		t.Fatal("symlink source root was accepted")
	}
}

func join(values []string) string {
	var result string
	for _, value := range values {
		result += value + "\n"
	}
	return result
}
