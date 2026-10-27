package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// listedPackage is the subset of `go list -json` this tool reads.
type listedPackage struct {
	ImportPath      string
	Name            string
	Dir             string
	Export          string
	DepOnly         bool
	Standard        bool
	GoFiles         []string
	CompiledGoFiles []string
	ImportMap       map[string]string
	Module          *struct {
		Path    string
		Version string
		Main    bool
	}
	Error *struct{ Err string }
}

// goList runs `go list -e -export -deps -json` in dir for patterns and
// returns the packages in dependency order (every package after its
// imports), which is the order go list prints -deps in.
//
// -export compiles every listed package so its export data can stand in for
// source when a package is only imported. -e keeps going past a package that
// fails to compile: the report still wants the call sites in the packages
// that do, and the type errors themselves name what broke.
func goList(dir string, env []string, patterns ...string) ([]*listedPackage, error) {
	args := append([]string{"list", "-e", "-export", "-deps", "-json"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var pkgs []*listedPackage
	dec := json.NewDecoder(&stdout)
	for {
		var p listedPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list in %s: decoding output: %w", dir, err)
		}
		pkgs = append(pkgs, &p)
	}
	if len(pkgs) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("go list %s in %s: %v\n%s", strings.Join(patterns, " "), dir, runErr, stderr.String())
		}
		return nil, fmt.Errorf("go list %s in %s matched no packages", strings.Join(patterns, " "), dir)
	}
	return pkgs, nil
}

// goCommand runs a go subcommand and returns its stdout.
func goCommand(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return out, nil
}
