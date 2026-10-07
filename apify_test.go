package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRoundTrip(t *testing.T) {
	t.Setenv("ACTOR_RUN_ID", "")
	c := &Client{dir: t.TempDir()}

	var in struct{ Keywords string }
	if err := c.SetValue("", "INPUT", map[string]string{"Keywords": "devops"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Input(&in); err != nil || in.Keywords != "devops" {
		t.Fatalf("input = %+v, err %v", in, err)
	}

	var v []string
	if ok, err := c.GetValue("state", "missing", &v); ok || err != nil {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}
	if err := c.SetValue("state", "k", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := c.GetValue("state", "k", &v); !ok || err != nil || v[0] != "a" {
		t.Fatalf("get = %v ok=%v err=%v", v, ok, err)
	}

	if err := c.PushData(map[string]int{"a": 1}, map[string]int{"a": 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "datasets", "default", "000000002.json")); err != nil {
		t.Fatal(err)
	}
	c2 := &Client{dir: c.dir} // a new process must not overwrite earlier items
	if err := c2.PushData(map[string]int{"a": 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "datasets", "default", "000000003.json")); err != nil {
		t.Fatal(err)
	}
}
