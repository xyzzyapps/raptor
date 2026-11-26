package raptor

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestAllPerlCookbookChapters(t *testing.T) {
	entries, err := os.ReadDir("../examples/cookbook")
	if err != nil {
		t.Fatalf("readdir cookbook: %v", err)
	}
	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".rp") {
			scripts = append(scripts, filepath.Join("../examples/cookbook", e.Name()))
		}
	}
	sort.Strings(scripts)
	if len(scripts) < 20 {
		t.Fatalf("expected all 20 Perl Cookbook chapters, found %d", len(scripts))
	}
	for _, path := range scripts {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			in := NewInterp()
			var out bytes.Buffer
			in.Stdout = &out
			in.Stderr = &out
			in.GlobalEnv.Define("$*PROGRAM", StringValue(path))
			if _, err := in.Eval(string(src)); err != nil {
				t.Fatalf("%s failed: %v\nOutput:\n%s", path, err, out.String())
			}
		})
	}
}

func TestAllMasteringAlgorithmsChapters(t *testing.T) {
	entries, err := os.ReadDir("../examples/algorithms")
	if err != nil {
		t.Fatalf("readdir algorithms: %v", err)
	}
	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".rp") {
			scripts = append(scripts, filepath.Join("../examples/algorithms", e.Name()))
		}
	}
	sort.Strings(scripts)
	if len(scripts) < 14 {
		t.Fatalf("expected all 14 Mastering Algorithms chapters, found %d", len(scripts))
	}
	for _, path := range scripts {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			in := NewInterp()
			var out bytes.Buffer
			in.Stdout = &out
			in.Stderr = &out
			in.GlobalEnv.Define("$*PROGRAM", StringValue(path))
			if _, err := in.Eval(string(src)); err != nil {
				t.Fatalf("%s failed: %v\nOutput:\n%s", path, err, out.String())
			}
		})
	}
}

func TestTodoWebAppSelfTest(t *testing.T) {
	t.Setenv("RAPTOR_TEST_MODE", "1")
	t.Setenv("RAPTOR_TODO_DB", ":memory:")
	src, err := os.ReadFile("../examples/todo/app.rp")
	if err != nil {
		t.Fatalf("read todo/app.rp: %v", err)
	}
	in := NewInterp()
	var out bytes.Buffer
	in.Stdout = &out
	in.Stderr = &out
	in.GlobalEnv.Define("$*PROGRAM", StringValue("../examples/todo/app.rp"))
	in.GlobalEnv.Define("@ARGV", ArrayValue([]*Value{StringValue("--self-test")}))
	if _, err := in.Eval(string(src)); err != nil {
		t.Fatalf("todo/app.rp failed: %v\nOutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Todo Web App self-test PASSED") {
		t.Fatalf("unexpected todo/app.rp output:\n%s", out.String())
	}
}
