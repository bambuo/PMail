package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func buildPluginTgz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPluginManagerInstall(t *testing.T) {
	root := t.TempDir()
	m := newPluginManager(root, "test")
	if err := m.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.stop()

	manifest := []byte(`{"id":"demo","name":"Demo","version":"1.0.0","abi":"pmail-hook@1","hooks":["GetName"],"entry":"plugin.wasm"}`)
	pkg := buildPluginTgz(t, map[string][]byte{
		"./manifest.json": manifest,
		"./plugin.wasm":   []byte("wasm"),
	})
	info, err := m.install(pkg)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if info.ID != "demo" || info.Status != "pending" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if _, err := os.Stat(filepath.Join(root, "demo", "manifest.json")); err != nil {
		t.Fatalf("manifest not installed: %v", err)
	}
	st, ok := m.store.Get("demo")
	if !ok || st.Status != "pending" || st.ManifestSHA256 == "" || st.WasmSHA256 == "" {
		t.Fatalf("unexpected state: %+v ok=%v", st, ok)
	}
	if _, err := m.install(pkg); err == nil {
		t.Fatal("expected duplicate install to fail")
	}
}

func TestPluginManagerInstallRejectsUnsafePackage(t *testing.T) {
	root := t.TempDir()
	m := newPluginManager(root, "test")
	if err := m.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.stop()

	manifest := []byte(`{"id":"demo","name":"Demo","version":"1.0.0","abi":"pmail-hook@1","hooks":["GetName"],"entry":"plugin.wasm"}`)
	cases := []map[string][]byte{
		{"manifest.json": manifest, "plugin.wasm": []byte("wasm"), "../escape": []byte("x")},
		{"manifest.json": manifest, "plugin.wasm": []byte("wasm"), "extra.txt": []byte("x")},
		{"manifest.json": manifest},
		{"manifest.json": []byte(`{"id":"../escape"}`), "plugin.wasm": []byte("wasm")},
	}
	for i, files := range cases {
		if _, err := m.install(buildPluginTgz(t, files)); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestPluginManagerInstallRejectsZipPackage(t *testing.T) {
	root := t.TempDir()
	m := newPluginManager(root, "test")
	if err := m.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.stop()

	if _, err := m.install([]byte("PK\x03\x04zip")); err == nil {
		t.Fatal("expected zip package to be rejected")
	}
}

func TestPluginManagerInstallIgnoresAppleDoubleMetadata(t *testing.T) {
	root := t.TempDir()
	m := newPluginManager(root, "test")
	if err := m.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.stop()

	manifest := []byte(`{"id":"demo","name":"Demo","version":"1.0.0","abi":"pmail-hook@1","hooks":["GetName"],"entry":"plugin.wasm"}`)
	pkg := buildPluginTgz(t, map[string][]byte{
		"./manifest.json":    manifest,
		"./plugin.wasm":      []byte("wasm"),
		"./._manifest.json":  []byte("apple-double"),
		"./._plugin.wasm":    []byte("apple-double"),
		"._.":                []byte("apple-double"),
		"./subdir/./ignored": nil,
	})
	if _, err := m.install(pkg); err == nil {
		t.Fatal("expected nested path to be rejected")
	}

	pkg = buildPluginTgz(t, map[string][]byte{
		"./manifest.json":   manifest,
		"./plugin.wasm":     []byte("wasm"),
		"./._manifest.json": []byte("apple-double"),
		"./._plugin.wasm":   []byte("apple-double"),
		"._.":               []byte("apple-double"),
	})
	if _, err := m.install(pkg); err != nil {
		t.Fatalf("install with AppleDouble metadata: %v", err)
	}
}
