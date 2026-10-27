// Command surface-coverage reports which of the SDK's operations a
// downstream Go repository — the CLI, the MCP server — reaches, which it
// does not, and which of its SDK references are stale.
//
// # Usage
//
//	go run ./scripts/surface-coverage -sdk v0.30.0 -repo ../basecamp-cli
//
// prints Markdown for a pull request body. -format json prints the report as
// JSON instead, and -json FILE writes the JSON alongside the Markdown.
//
// Flags:
//
//	-sdk VERSION    the SDK release to report against: a tag, "main", or
//	                "latest" (the default). Fetched through the Go module proxy.
//	-sdk-dir DIR    report against a local SDK checkout instead (the repository
//	                root or its go/ module). -sdk then only labels it.
//	-prev VERSION   the release to compute the delta against. Defaults to the
//	                newest release before -sdk; "none" skips the delta.
//	-repo DIR       the downstream repository (default ".").
//	-ref REF        analyze this git ref of -repo (default: the working tree).
//	-pkg PATTERNS   comma-separated package patterns that make up the shipped
//	                code. Defaults to ./cmd/basecamp for basecamp-cli,
//	                ./cmd/basecamp-mcp for basecamp-mcp-server, else ./... .
//
// # How coverage is derived
//
// From the code, never from a list. Each side is type-checked from source:
//
//   - The SDK at -sdk: every function and method in go/pkg/... is mapped to the
//     operations it performs — the generated-client entry points it reaches
//     (CreateTodoWithResponse is CreateTodo), and the raw HTTP calls whose
//     path matches an operation's route — followed through the SDK's own
//     internal calls, so TodosService.Update reaches GetTodo and ReplaceTodo.
//     The operation list is the SDK's embedded catalog
//     (go/pkg/basecamp/catalog/catalog.json), which `make catalog-check`
//     holds equal to openapi.json.
//   - The downstream repository: every reference its non-test packages make
//     to an SDK function or method, typed against the SDK version its go.mod
//     pins, plus the paths it passes to the SDK's raw Get/Post/Put/Delete.
//
// An operation is reached when any reference leads to it. A reference to a
// symbol the reported SDK does not declare, or to an operation it no longer
// has, is stale. The delta compares against the previous release: the
// operations it added that the repository does not reach, and operations the
// repository reached through the previous release's SDK but not this one's.
//
// The tool needs the go toolchain and, without -sdk-dir, the module proxy.
// It writes nothing outside the system temp directory.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "surface-coverage:", err)
		os.Exit(1)
	}
}

type options struct {
	sdk, sdkDir, prev, prevDir, repo, ref, pkg, format, jsonOut string
}

func run(args []string, stdout io.Writer) error {
	var o options
	fs := flag.NewFlagSet("surface-coverage", flag.ContinueOnError)
	fs.StringVar(&o.sdk, "sdk", "latest", "SDK release: a tag, main, or latest")
	fs.StringVar(&o.sdkDir, "sdk-dir", "", "local SDK checkout to report against instead of the module proxy")
	fs.StringVar(&o.prev, "prev", "", `previous release for the delta (default: the newest before -sdk; "none" to skip)`)
	fs.StringVar(&o.prevDir, "prev-dir", "", "local SDK checkout for the previous release")
	fs.StringVar(&o.repo, "repo", ".", "downstream repository")
	fs.StringVar(&o.ref, "ref", "", "git ref of -repo to analyze (default: the working tree)")
	fs.StringVar(&o.pkg, "pkg", "", "comma-separated package patterns of the shipped code")
	fs.StringVar(&o.format, "format", "markdown", "markdown or json")
	fs.StringVar(&o.jsonOut, "json", "", "also write the JSON report to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.format != "markdown" && o.format != "json" {
		return fmt.Errorf("-format must be markdown or json, not %q", o.format)
	}

	tmp, err := os.MkdirTemp("", "surface-coverage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	// Resolve symlinks (macOS's /var is /private/var) so the file names the
	// type checker reports are relative to the same root the report uses.
	if tmp, err = filepath.EvalSymlinks(tmp); err != nil {
		return err
	}

	sdk, err := resolveSDK(o.sdk, o.sdkDir, tmp)
	if err != nil {
		return err
	}

	var prev *SDK
	switch {
	case o.prevDir != "":
		prev, err = resolveSDK(o.prev, o.prevDir, tmp)
	case o.prev == "none":
	default:
		v := o.prev
		if v == "" && o.sdkDir != "" {
			v = localPrevious(o.sdkDir)
		}
		if v == "" {
			if v, err = previousVersion(sdk.Version, tmp); err != nil {
				return err
			}
		}
		if v != "" {
			prev, err = resolveSDK(v, "", tmp)
		}
	}
	if err != nil {
		return err
	}

	repoDir, commit, err := checkoutRef(o.repo, o.ref, tmp)
	if err != nil {
		return err
	}
	var patterns []string
	if o.pkg != "" {
		patterns = strings.Split(o.pkg, ",")
	}
	repo, err := loadRepo(repoDir, patterns)
	if err != nil {
		return err
	}

	pinned := sdk
	switch repo.PinnedSDK {
	case "", sdk.Version:
	default:
		if prev != nil && repo.PinnedSDK == prev.Version {
			pinned = prev
		} else if pinned, err = resolveSDK(repo.PinnedSDK, "", tmp); err != nil {
			return fmt.Errorf("loading the SDK the repository pins: %w", err)
		}
	}

	rep := buildReport(sdk, prev, pinned, repo)
	rep.Ref, rep.Commit = o.ref, commit

	if o.jsonOut != "" {
		if err := writeJSON(o.jsonOut, rep); err != nil {
			return err
		}
	}
	if o.format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	renderMarkdown(stdout, rep)
	return nil
}

func writeJSON(path string, rep *Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// proxyEnv runs module downloads outside any workspace or module.
var proxyEnv = []string{"GOWORK=off", "GOFLAGS=-mod=mod"}

// resolveSDK loads the SDK at version — through the module proxy, or from
// dir when one is given.
func resolveSDK(version, dir, tmp string) (*SDK, error) {
	if dir != "" {
		mod, err := sdkModuleDir(dir)
		if err != nil {
			return nil, err
		}
		if version == "" || version == "latest" {
			version = describe(dir)
		}
		return loadSDK(mod, version)
	}
	out, err := goCommand(tmp, proxyEnv, "mod", "download", "-json", sdkModulePath+"@"+version)
	if err != nil {
		return nil, err
	}
	var dl struct{ Version, Dir, Error string }
	if err := json.Unmarshal(out, &dl); err != nil {
		return nil, err
	}
	if dl.Error != "" {
		return nil, fmt.Errorf("downloading SDK %s: %s", version, dl.Error)
	}
	return loadSDK(dl.Dir, dl.Version)
}

func previousVersion(v, tmp string) (string, error) {
	out, err := goCommand(tmp, proxyEnv, "list", "-m", "-versions", "-json", sdkModulePath)
	if err != nil {
		return "", err
	}
	var m struct{ Versions []string }
	if err := json.Unmarshal(out, &m); err != nil {
		return "", err
	}
	return previousRelease(v, m.Versions), nil
}

// checkoutRef returns the directory to analyze and the commit it holds. With
// a ref, that is a fresh export of the ref's tree under tmp, so the
// repository's working tree is never touched.
func checkoutRef(repo, ref, tmp string) (string, string, error) {
	abs, err := filepath.Abs(repo)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return "", "", err
	}
	if ref == "" {
		return abs, workingTreeLabel(abs), nil
	}
	commit := gitShort(abs, ref+"^{commit}")
	if commit == "" {
		return "", "", fmt.Errorf("%s has no ref %q", repo, ref)
	}
	dst := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", "", err
	}
	archive := exec.Command("git", "-C", abs, "archive", "--format=tar", commit)
	extract := exec.Command("tar", "-x", "-C", dst)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	extract.Stdin = pipe
	var stderr strings.Builder
	archive.Stderr, extract.Stderr = &stderr, &stderr
	if err := extract.Start(); err != nil {
		return "", "", err
	}
	if err := archive.Run(); err != nil {
		return "", "", fmt.Errorf("git archive %s: %v %s", ref, err, stderr.String())
	}
	if err := extract.Wait(); err != nil {
		return "", "", fmt.Errorf("extracting %s: %v %s", ref, err, stderr.String())
	}
	return dst, commit, nil
}

// workingTreeLabel names what an analysis of the working tree read: the HEAD
// commit, marked "-dirty" when the tree has changes the commit does not, so
// a report never claims a commit's coverage for code that is not in it.
func workingTreeLabel(dir string) string {
	head := gitShort(dir, "HEAD")
	if head == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "--untracked-files=normal").Output()
	if err != nil || len(strings.TrimSpace(string(out))) > 0 {
		return head + "-dirty"
	}
	return head
}

func gitShort(dir, rev string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short=12", "--verify", "--quiet", rev).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// The Go module's releases are the go/v* tags; a v* tag alone is not one.
const goTagPrefix = "go/"

// describe labels a local checkout by its nearest Go module release:
// "v0.29.1" when it is that release, "v0.29.1-3-gabc123" when ahead of it.
func describe(dir string) string {
	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--match", goTagPrefix+"v*").Output()
	if err != nil {
		if short := gitShort(dir, "HEAD"); short != "" {
			return short
		}
		return "local"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), goTagPrefix)
}

// localPrevious is the release a local checkout's delta is against: the
// nearest Go module release when the checkout is ahead of it, or "" (the
// release before, from the module proxy) when it is that release.
func localPrevious(dir string) string {
	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--abbrev=0", "--match", goTagPrefix+"v*").Output()
	if err != nil {
		return ""
	}
	tag := strings.TrimPrefix(strings.TrimSpace(string(out)), goTagPrefix)
	if tag == describe(dir) {
		return ""
	}
	return tag
}
