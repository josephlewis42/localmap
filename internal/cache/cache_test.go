package cache

import (
	"os"
	"testing"
	"time"
)

func TestCacheSaveLoad(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 5*time.Minute)
	key := "width=32&height=24"
	data := []byte("pngdata")

	if err := c.Save(key, data); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, ok, err := c.Load(key)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != string(data) {
		t.Fatalf("unexpected data: got %q", got)
	}

	path := c.cachePath(key)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file exists: %v", err)
	}
}

func TestCacheExpires(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 1*time.Millisecond)
	key := "width=24&height=24"
	data := []byte("pngdata")

	if err := c.Save(key, data); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	_, ok, err := c.Load(key)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestCleanupRemovesExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 1*time.Millisecond)
	key := "foo=bar"
	if err := c.Save(key, []byte("data")); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	if err := c.Cleanup(); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	path := c.cachePath(key)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected expired file removed, got %v", err)
	}
}

func TestMakeKeyDeterministic(t *testing.T) {
	a := MakeKey(map[string]string{"b": "2", "a": "1"})
	b := MakeKey(map[string]string{"a": "1", "b": "2"})
	if a != b {
		t.Fatalf("expected same key; got %q and %q", a, b)
	}
	if a != "a=1&b=2" {
		t.Fatalf("unexpected key: %q", a)
	}
}
