// Command coverage measures, per API object, how many of its JSON-RPC methods
// the SDK implements, and writes a human-readable report to api-spec/COVERAGE.md.
//
// Usage: go run ./api-spec/coverage
//
// It reads the committed OpenRPC snapshot (api-spec/openrpc/*.json) for the set
// of objects and their method names, then inspects the SDK source to decide
// which methods are implemented.
//
// # Detection heuristic
//
// This is a heuristic, not a proof of behaviour. The SDK dispatches every
// JSON-RPC call through a single transport, s.c.call(ctx, <endpointConst>,
// "<method>", ...), where <endpointConst> is a package const such as
// vpsEndpoint = "/vps". The tool:
//
//  1. maps each endpoint path to its const identifier by scanning the SDK for
//     `<name>Endpoint = "/path"` declarations;
//  2. finds every SDK .go file that references that const identifier (one object
//     can span files — the /vps methods live in vps/vps.go and vps/config.go);
//  3. collects the Go string literals in those files (via go/scanner, so
//     comments and identifiers are excluded);
//  4. counts a spec method as implemented when its exact name appears among
//     those literals.
//
// Scoping the literal search to the files that use the object's endpoint const
// (rather than the whole repo) is deliberate: it keeps generic wire names like
// "create" or "index" from matching across unrelated objects. The wire method
// name is always a literal somewhere in those files, even for methods dispatched
// through a helper that receives the name as a parameter (e.g. powerOn/powerOff/
// reboot via VPSService.powerAction).
//
// Known limitations:
//   - False positive: a method name that appears as a literal for an unrelated
//     reason (a params key, an error string) in a file using the endpoint const
//     would be counted as implemented.
//   - False negative: an object whose endpoint path has no const, or whose
//     methods are dispatched from a file that never names the const, reads as 0%.
//
// This is provenance/roadmap tooling; it is not part of the importable SDK.
package main

import (
	"encoding/json"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	specDir      = "api-spec/openrpc"
	sdkDir       = "."
	reportPath   = "api-spec/COVERAGE.md"
	apiHost      = "https://api.sweb.ru"
	endpointFrag = "Endpoint = \""
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "coverage:", err)
		os.Exit(1)
	}
}

// object is one API object from the snapshot with its implementation status.
type object struct {
	path    string   // endpoint path, e.g. "/vps"
	methods []string // spec method names, in spec order
	missing []string // spec methods with no SDK literal, sorted
	covered int      // count implemented
}

func run() error {
	objects, err := loadObjects(specDir)
	if err != nil {
		return err
	}

	pathToLiterals, err := sdkLiteralsByEndpoint(sdkDir)
	if err != nil {
		return err
	}

	for i := range objects {
		o := &objects[i]
		impl := pathToLiterals[o.path] // nil set for unmapped objects → 0%
		for _, m := range o.methods {
			if impl[m] {
				o.covered++
			} else {
				o.missing = append(o.missing, m)
			}
		}
		sort.Strings(o.missing)
	}

	// Stable order: by endpoint path, so the report diffs cleanly.
	sort.Slice(objects, func(a, b int) bool { return objects[a].path < objects[b].path })

	return writeReport(reportPath, objects)
}

// loadObjects reads every OpenRPC document under dir and returns one object per
// file, keyed by its servers[0].url path.
func loadObjects(dir string) ([]object, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read spec dir: %w", err)
	}
	var objects []object
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", full, err)
		}
		var spec struct {
			Servers []struct {
				URL string `json:"url"`
			} `json:"servers"`
			Methods []struct {
				Name string `json:"name"`
			} `json:"methods"`
		}
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parse %s: %w", full, err)
		}
		path := ""
		if len(spec.Servers) > 0 {
			path = strings.TrimPrefix(spec.Servers[0].URL, apiHost)
		}
		methods := make([]string, 0, len(spec.Methods))
		for _, m := range spec.Methods {
			if m.Name != "" {
				methods = append(methods, m.Name)
			}
		}
		objects = append(objects, object{path: path, methods: methods})
	}
	return objects, nil
}

// sdkLiteralsByEndpoint maps each endpoint path to the set of Go string literals
// found across the SDK files that reference that path's endpoint const.
//
// It works in three passes over the SDK .go files across the module (excluding
// tests and the api-spec/ tree): find endpoint const declarations (path → const name),
// find which files reference each const identifier, then union the string
// literals of those files per path.
func sdkLiteralsByEndpoint(dir string) (map[string]map[string]bool, error) {
	files, err := sdkGoFiles(dir)
	if err != nil {
		return nil, err
	}

	sources := make(map[string][]byte, len(files))
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		sources[f] = src
	}

	// Pass 1: endpoint path -> const identifier (e.g. "/vps" -> "vpsEndpoint").
	pathConst := map[string]string{}
	for f, src := range sources {
		for path, name := range endpointConsts(src) {
			if prior, ok := pathConst[path]; ok && prior != name {
				return nil, fmt.Errorf("conflicting endpoint const for %q: %s and %s (in %s)", path, prior, name, f)
			}
			pathConst[path] = name
		}
	}

	// Pass 2 & 3: for each const, union the string literals of every file that
	// references the const identifier.
	result := map[string]map[string]bool{}
	for path, constName := range pathConst {
		lits := map[string]bool{}
		for _, src := range sources {
			if !identifierUsed(src, constName) {
				continue
			}
			for lit := range stringLiterals(src) {
				lits[lit] = true
			}
		}
		result[path] = lits
	}
	return result, nil
}

// sdkGoFiles lists the SDK .go source files, walking the whole module tree,
// skipping _test.go files and anything under the api-spec/ tree (spec tooling,
// not the SDK surface). Since the per-service restructure the endpoint consts
// and their method literals live in service subpackages (vps/, ip/, …), so the
// walk must recurse rather than read only the root.
func sdkGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip the spec tooling tree and hidden dirs (e.g. .git).
			if path != dir && (d.Name() == "api-spec" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk sdk dir: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

// endpointConsts extracts `<name>Endpoint = "/path"` declarations from Go source
// via the scanner, returning path -> const identifier. Using the token stream
// (rather than a text regexp) means only real const string values are matched.
func endpointConsts(src []byte) map[string]string {
	out := map[string]string{}
	toks := tokenize(src)
	for i := 0; i+2 < len(toks); i++ {
		if toks[i].tok != token.IDENT || !strings.HasSuffix(toks[i].lit, "Endpoint") {
			continue
		}
		if toks[i+1].tok != token.ASSIGN {
			continue
		}
		if toks[i+2].tok != token.STRING {
			continue
		}
		path, err := strconv.Unquote(toks[i+2].lit)
		if err != nil {
			continue
		}
		out[path] = toks[i].lit
	}
	return out
}

// identifierUsed reports whether name appears as an identifier token in src.
func identifierUsed(src []byte, name string) bool {
	for _, t := range tokenize(src) {
		if t.tok == token.IDENT && t.lit == name {
			return true
		}
	}
	return false
}

// stringLiterals returns the set of decoded Go string literal values in src.
// Comments are not tokenized as strings, so method names mentioned in comments
// do not leak in.
func stringLiterals(src []byte) map[string]bool {
	out := map[string]bool{}
	for _, t := range tokenize(src) {
		if t.tok != token.STRING {
			continue
		}
		if v, err := strconv.Unquote(t.lit); err == nil {
			out[v] = true
		}
	}
	return out
}

// tok is one scanned token: kind and its literal text.
type scanned struct {
	tok token.Token
	lit string
}

// tokenize runs the Go scanner over src and returns its token stream. Scan
// errors are ignored (best-effort over committed, compiling source).
func tokenize(src []byte) []scanned {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, src, nil, 0)
	var out []scanned
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		out = append(out, scanned{tok: tok, lit: lit})
	}
	return out
}

// writeReport writes the deterministic Markdown coverage report.
func writeReport(path string, objects []object) error {
	var b strings.Builder

	b.WriteString("# SDK API coverage\n\n")
	b.WriteString("How many JSON-RPC methods of each API object the SDK implements, ")
	b.WriteString("measured live from the OpenRPC snapshot (`api-spec/openrpc/`) and ")
	b.WriteString("the SDK source. Generated by `mise run coverage` ")
	b.WriteString("(`go run ./api-spec/coverage`) — do not edit by hand.\n\n")
	b.WriteString("Detection is a heuristic: a spec method counts as implemented when ")
	b.WriteString("its name appears as a Go string literal in an SDK file that ")
	b.WriteString("references the object's endpoint const. See `api-spec/coverage/main.go` ")
	b.WriteString("for the method and its false-positive/negative caveats.\n\n")

	var totalCovered, totalMethods int
	for _, o := range objects {
		totalCovered += o.covered
		totalMethods += len(o.methods)
	}
	fmt.Fprintf(&b, "**Overall: %d/%d methods (%s) across %d objects.**\n\n",
		totalCovered, totalMethods, pct(totalCovered, totalMethods), len(objects))

	b.WriteString("| Object | Covered | Missing methods |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, o := range objects {
		missing := "—"
		if len(o.missing) > 0 {
			missing = "`" + strings.Join(o.missing, "`, `") + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %d/%d (%s) | %s |\n",
			o.path, o.covered, len(o.methods), pct(o.covered, len(o.methods)), missing)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s — %d/%d methods (%s) across %d objects\n",
		path, totalCovered, totalMethods, pct(totalCovered, totalMethods), len(objects))
	return nil
}

// pct formats an integer-ratio percentage; 0/0 reads as 0%.
func pct(n, d int) string {
	if d == 0 {
		return "0%"
	}
	return strconv.Itoa(n*100/d) + "%"
}
