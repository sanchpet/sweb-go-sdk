// Command gen regenerates the OpenRPC snapshot under api-spec/openrpc/.
//
// SpaceWeb publishes no machine-readable API spec. Its documentation site
// (apidoc.sweb.ru) is a create-react-app SPA that inlines one OpenRPC document
// per API object into its JavaScript bundle (webpack `JSON.parse("…")` blobs).
// This tool resolves the bundle chunks via the site's asset-manifest, scans
// them for those blobs, and writes each OpenRPC document as a pretty-printed,
// key-sorted JSON file so the snapshot diffs cleanly when upstream changes.
//
// Usage: go run ./api-spec/gen
//
// The output is deterministic: object keys are sorted (encoding/json) and files
// are named by the object's server URL path. This is provenance-only tooling;
// it is not part of the importable SDK surface.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	baseURL      = "https://apidoc.sweb.ru"
	manifestPath = "/asset-manifest.json"
	apiHost      = "https://api.sweb.ru"
	outDir       = "api-spec/openrpc"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	chunks, err := chunkURLs()
	if err != nil {
		return err
	}
	var blob strings.Builder
	for _, u := range chunks {
		body, err := fetch(u)
		if err != nil {
			return fmt.Errorf("fetch chunk %s: %w", u, err)
		}
		blob.Write(body)
		blob.WriteByte('\n')
	}

	docs, err := extract(blob.String())
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return fmt.Errorf("no OpenRPC documents found — bundle layout may have changed")
	}

	if err := writeDocs(docs); err != nil {
		return err
	}
	fmt.Printf("wrote %d OpenRPC documents to %s/\n", len(docs), outDir)
	return nil
}

// chunkURLs resolves the JavaScript chunk URLs from the site's asset-manifest,
// so a rebuild that renames chunks is followed automatically.
func chunkURLs() ([]string, error) {
	body, err := fetch(baseURL + manifestPath)
	if err != nil {
		return nil, fmt.Errorf("fetch asset-manifest: %w", err)
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse asset-manifest: %w", err)
	}
	seen := map[string]bool{}
	var urls []string
	for _, v := range manifest.Files {
		if !strings.HasSuffix(v, ".js") {
			continue
		}
		u := v
		if strings.HasPrefix(u, "/") {
			u = baseURL + u
		}
		if !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	sort.Strings(urls)
	if len(urls) == 0 {
		return nil, fmt.Errorf("asset-manifest lists no .js chunks")
	}
	return urls, nil
}

// doc is one extracted OpenRPC document plus the metadata used to name it.
type doc struct {
	slug    string
	methods int
	first   string
	bytes   []byte // normalized, key-sorted JSON (no trailing newline)
}

// extract scans concatenated bundle text for `JSON.parse("…")` string literals,
// double-decodes each (JS string literal → inner JSON), and keeps those that are
// OpenRPC documents.
func extract(src string) ([]doc, error) {
	const needle = `JSON.parse(`
	var out []doc
	seenBytes := map[string]bool{}

	for i := 0; i < len(src); {
		idx := strings.Index(src[i:], needle)
		if idx < 0 {
			break
		}
		p := i + idx + len(needle)
		if p >= len(src) || src[p] != '"' {
			i = p
			continue
		}
		// Scan the JS double-quoted string literal, honoring backslash escapes.
		j := p + 1
		for j < len(src) {
			if src[j] == '\\' {
				j += 2
				continue
			}
			if src[j] == '"' {
				break
			}
			j++
		}
		if j >= len(src) {
			break
		}
		lit := src[p : j+1] // includes the surrounding quotes
		i = j + 1

		var inner string
		if err := json.Unmarshal([]byte(lit), &inner); err != nil {
			continue
		}
		obj, ok := decodeObject(inner)
		if !ok {
			continue
		}
		if _, ok := obj["openrpc"]; !ok {
			if _, ok := obj["methods"]; !ok {
				continue
			}
		}
		norm, err := marshal(obj)
		if err != nil {
			continue
		}
		if seenBytes[string(norm)] {
			continue // same document inlined twice
		}
		seenBytes[string(norm)] = true
		out = append(out, doc{
			slug:    slugOf(obj),
			methods: methodCount(obj),
			first:   firstMethod(obj),
			bytes:   norm,
		})
	}
	return out, nil
}

// decodeObject decodes JSON into a generic object, keeping numbers verbatim so
// re-marshaling does not reformat them.
func decodeObject(s string) (map[string]any, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, false
	}
	return obj, true
}

func marshal(obj map[string]any) ([]byte, error) {
	// encoding/json sorts object keys, giving a stable, diff-friendly layout.
	return json.MarshalIndent(obj, "", "  ")
}

// slugOf derives a filename stem from the document's first server URL path,
// e.g. https://api.sweb.ru/vps/ip -> "vps-ip".
func slugOf(obj map[string]any) string {
	servers, _ := obj["servers"].([]any)
	if len(servers) > 0 {
		if s, ok := servers[0].(map[string]any); ok {
			if url, ok := s["url"].(string); ok {
				path := strings.TrimPrefix(url, apiHost)
				path = strings.TrimPrefix(path, "/")
				slug := strings.ToLower(strings.ReplaceAll(path, "/", "-"))
				if slug != "" {
					return slug
				}
			}
		}
	}
	return "object"
}

func methodCount(obj map[string]any) int {
	m, _ := obj["methods"].([]any)
	return len(m)
}

func firstMethod(obj map[string]any) string {
	methods, _ := obj["methods"].([]any)
	if len(methods) > 0 {
		if m, ok := methods[0].(map[string]any); ok {
			if n, ok := m["name"].(string); ok {
				return n
			}
		}
	}
	return ""
}

// writeDocs empties the output directory and writes one file per document.
// Slug collisions (a single object path serving two documents) are resolved
// deterministically by suffixing all but the largest with an index.
func writeDocs(docs []doc) error {
	// Stable order: by slug, then larger method sets first, then first method.
	sort.Slice(docs, func(a, b int) bool {
		if docs[a].slug != docs[b].slug {
			return docs[a].slug < docs[b].slug
		}
		if docs[a].methods != docs[b].methods {
			return docs[a].methods > docs[b].methods
		}
		return docs[a].first < docs[b].first
	})

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := cleanJSON(outDir); err != nil {
		return err
	}

	groupSeen := map[string]int{}
	for _, d := range docs {
		name := d.slug
		if n := groupSeen[d.slug]; n > 0 {
			name = fmt.Sprintf("%s-%d", d.slug, n+1)
		}
		groupSeen[d.slug]++

		path := filepath.Join(outDir, name+".json")
		content := append(append([]byte{}, d.bytes...), '\n')
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cleanJSON(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sweb-go-sdk-apispec-gen")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
