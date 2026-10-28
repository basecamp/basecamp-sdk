package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fixtures is a temp copy of testdata in which each fake SDK's
// generated-client/ stands at pkg/generated, the import path the tool
// recognizes. The repository keeps no hand-written file under a generated/
// directory (AGENTS.md, "Never Do These"), so the tree is assembled here.
var fixtures string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "surface-coverage-fixtures-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dir, err = filepath.EvalSymlinks(dir); err == nil {
		err = assembleFixtures("testdata", dir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "assembling fixtures:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	fixtures = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func assembleFixtures(src, dst string) error {
	if err := copyDir(src, dst); err != nil {
		return err
	}
	for _, v := range []string{"sdk-v1", "sdk-v2"} {
		from := filepath.Join(dst, v, "generated-client")
		to := filepath.Join(dst, v, "pkg", "generated")
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}

func fixture(parts ...string) string {
	return filepath.Join(append([]string{fixtures}, parts...)...)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
