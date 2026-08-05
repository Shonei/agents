package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Shonei/agents/pkg/config"
	"github.com/Shonei/agents/pkg/sdk"
	"github.com/Shonei/agents/pkg/utils"
)

const (
	specFormatOpenAPI3 = "openapi_3"
	specFormatOpenAPI2 = "openapi_2"
	specFormatPostman  = "postman"
	defaultMaxOps      = 250
)

// IngestAPISpecTool fetches or reads an OpenAPI/Swagger or Postman Collection
// document and returns a structured summary the agent can turn into local docs.
type IngestAPISpecTool struct{}

func (t *IngestAPISpecTool) Name() string {
	return "ingest_api_spec"
}

func (t *IngestAPISpecTool) Description() string {
	return "Fetches or reads a machine-readable API spec (OpenAPI 2/3 / Swagger, or Postman Collection) and returns a structured summary: servers, security schemes, and operations (method, path, summary, parameters, responses, tags). Prefer this over scraping HTML when a vendor publishes a spec. Pass either `url` or `path`. Optional `path_prefix` / `tag` filters and `max_operations` (default 250) keep large specs manageable."
}

func (t *IngestAPISpecTool) Init(_ map[string]string, _ *config.ConfigFactory) {
}

func (t *IngestAPISpecTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "HTTP(S) URL of an OpenAPI/Swagger JSON/YAML or Postman Collection JSON.",
				"example":     "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Local filesystem path to a spec file (relative or absolute).",
				"example":     "./openapi.yaml",
			},
			"path_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Only include operations whose path starts with this prefix (OpenAPI) or whose request URL path contains it (Postman).",
				"example":     "/orders",
			},
			"tag": map[string]interface{}{
				"type":        "string",
				"description": "Only include operations that have this OpenAPI tag (case-insensitive). Ignored for Postman.",
				"example":     "Orders",
			},
			"max_operations": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum operations to return (default 250). Remaining are counted in truncated_operations.",
				"example":     100,
			},
		},
	}
}

type ingestAPISpecInput struct {
	URL           string `json:"url"`
	Path          string `json:"path"`
	PathPrefix    string `json:"path_prefix"`
	Tag           string `json:"tag"`
	MaxOperations int    `json:"max_operations"`
}

// SpecIngestResult is the structured tool output.
type SpecIngestResult struct {
	Format              string              `json:"format"`
	Title               string              `json:"title,omitempty"`
	Version             string              `json:"version,omitempty"`
	Description         string              `json:"description,omitempty"`
	Source              string              `json:"source"`
	Servers             []string            `json:"servers,omitempty"`
	SecuritySchemes     []SpecSecurity      `json:"security_schemes,omitempty"`
	OperationCount      int                 `json:"operation_count"`
	TruncatedOperations int                 `json:"truncated_operations,omitempty"`
	Operations          []SpecOperation     `json:"operations"`
	SuggestedGroups     map[string][]string `json:"suggested_groups,omitempty"`
	Notes               []string            `json:"notes,omitempty"`
}

// SpecSecurity summarizes an auth scheme from the spec.
type SpecSecurity struct {
	Name         string `json:"name"`
	Type         string `json:"type,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	In           string `json:"in,omitempty"`
	BearerFormat string `json:"bearer_format,omitempty"`
	Description  string `json:"description,omitempty"`
}

// SpecOperation is one API operation suitable for a docs endpoint file.
type SpecOperation struct {
	ID              string            `json:"id"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Summary         string            `json:"summary,omitempty"`
	Description     string            `json:"description,omitempty"`
	OperationID     string            `json:"operation_id,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Deprecated      bool              `json:"deprecated,omitempty"`
	Parameters      []SpecParam       `json:"parameters,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	Responses       map[string]string `json:"responses,omitempty"`
	Security        []string          `json:"security,omitempty"`
	SuggestedFile   string            `json:"suggested_file,omitempty"`
	SuggestedGroup  string            `json:"suggested_group,omitempty"`
	ExternalDocsURL string            `json:"external_docs_url,omitempty"`
}

// SpecParam is a request parameter summary.
type SpecParam struct {
	Name        string `json:"name"`
	In          string `json:"in,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
	SchemaType  string `json:"type,omitempty"`
}

func (t *IngestAPISpecTool) Call(input map[string]interface{}) (interface{}, error) {
	var in ingestAPISpecInput
	if err := mapstruct(input, &in); err != nil {
		return "", err
	}

	if in.URL == "" && in.Path == "" {
		return "", sdk.NewAIError("either url or path is required")
	}
	if in.URL != "" && in.Path != "" {
		return "", sdk.NewAIError("provide only one of url or path, not both")
	}

	maxOps := in.MaxOperations
	if maxOps <= 0 {
		maxOps = defaultMaxOps
	}

	var (
		raw    []byte
		source string
		err    error
	)

	if in.Path != "" {
		raw, source, err = readSpecFile(in.Path)
	} else {
		raw, source, err = fetchSpecURL(in.URL)
	}
	if err != nil {
		return "", err
	}

	result, err := parseAPISpec(raw, source, in.PathPrefix, in.Tag, maxOps)
	if err != nil {
		return "", sdk.NewAIError(err.Error())
	}

	return result, nil
}

func readSpecFile(path string) ([]byte, string, error) {
	if path == "" {
		return nil, "", sdk.NewAIError("path is required")
	}

	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get current directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read spec file: %w", err)
	}

	return raw, path, nil
}

func fetchSpecURL(url string) ([]byte, string, error) {
	if url == "" {
		return nil, "", sdk.NewAIError("url is required")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	var content string
	err := utils.NewHTTPBuilder(url).
		New().
		WithHeader("User-Agent", "Mozilla/5.0 (compatible; agents-ingest-api-spec/1.0)").
		WithHeader("Accept", "application/json, application/yaml, text/yaml, text/plain, */*").
		Into(&content).
		Do()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch spec URL: %w", err)
	}

	return []byte(content), url, nil
}

func parseAPISpec(raw []byte, source, pathPrefix, tagFilter string, maxOps int) (*SpecIngestResult, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("spec document is empty")
	}

	var doc map[string]interface{}
	if err := decodeSpec(raw, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON/YAML spec: %w", err)
	}

	switch {
	case isPostman(doc):
		return parsePostman(doc, source, pathPrefix, maxOps)
	case strField(doc, "openapi") != "":
		return parseOpenAPI(doc, source, pathPrefix, tagFilter, maxOps, specFormatOpenAPI3)
	case strField(doc, "swagger") != "":
		return parseOpenAPI(doc, source, pathPrefix, tagFilter, maxOps, specFormatOpenAPI2)
	default:
		return nil, fmt.Errorf("unrecognized spec: expected OpenAPI (openapi/swagger key) or Postman Collection (info.schema)")
	}
}

func decodeSpec(raw []byte, out *map[string]interface{}) error {
	trimmed := bytesTrimLeftSpace(raw)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		if err := json.Unmarshal(raw, out); err == nil {
			return nil
		}
	}

	if err := yaml.Unmarshal(raw, out); err != nil {
		// Last resort: try JSON even if it didn't look like it.
		if jerr := json.Unmarshal(raw, out); jerr != nil {
			return err
		}
	}

	return nil
}

func bytesTrimLeftSpace(b []byte) []byte {
	return []byte(strings.TrimLeft(string(b), " \t\r\n"))
}

func isPostman(doc map[string]interface{}) bool {
	info, _ := doc["info"].(map[string]interface{})
	if info == nil {
		return false
	}
	schema := strField(info, "schema")

	return strings.Contains(strings.ToLower(schema), "postman")
}

func parseOpenAPI(doc map[string]interface{}, source, pathPrefix, tagFilter string, maxOps int, format string) (*SpecIngestResult, error) {
	info, _ := doc["info"].(map[string]interface{})
	result := &SpecIngestResult{
		Format:          format,
		Source:          source,
		Title:           strField(info, "title"),
		Version:         strField(info, "version"),
		Description:     truncateStr(strField(info, "description"), 500),
		SuggestedGroups: map[string][]string{},
		Notes: []string{
			"Parsed from OpenAPI/Swagger. Use operations to write endpoint files; use url_context on human docs for quirks/workflows.",
		},
	}

	result.Servers = openAPIServers(doc, format)
	result.SecuritySchemes = openAPISecuritySchemes(doc, format)

	paths, _ := doc["paths"].(map[string]interface{})
	if paths == nil {
		return nil, fmt.Errorf("OpenAPI document has no paths")
	}

	pathKeys := sortedKeys(paths)
	tagFilterLower := strings.ToLower(strings.TrimSpace(tagFilter))
	var matched []SpecOperation

	for _, p := range pathKeys {
		if pathPrefix != "" && !strings.HasPrefix(p, pathPrefix) {
			continue
		}
		item, _ := paths[p].(map[string]interface{})
		if item == nil {
			continue
		}

		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
			opRaw, ok := item[method]
			if !ok {
				continue
			}
			op, _ := opRaw.(map[string]interface{})
			if op == nil {
				continue
			}

			tags := stringSlice(op["tags"])
			if tagFilterLower != "" && !containsFold(tags, tagFilterLower) {
				continue
			}

			params := mergeParams(item["parameters"], op["parameters"])
			group := suggestedGroup(tags, p)
			opID := strField(op, "operationId")
			id := opID
			if id == "" {
				id = strings.ToLower(method) + "_" + slugPath(p)
			}

			extDocs := ""
			if ed, ok := op["externalDocs"].(map[string]interface{}); ok {
				extDocs = strField(ed, "url")
			}

			matched = append(matched, SpecOperation{
				ID:              id,
				Method:          strings.ToUpper(method),
				Path:            p,
				Summary:         strField(op, "summary"),
				Description:     truncateStr(strField(op, "description"), 800),
				OperationID:     opID,
				Tags:            tags,
				Deprecated:      boolField(op, "deprecated"),
				Parameters:      params,
				RequestBody:     summarizeRequestBody(op["requestBody"], format),
				Responses:       summarizeResponses(op["responses"]),
				Security:        summarizeSecurity(op["security"], doc["security"]),
				SuggestedGroup:  group,
				SuggestedFile:   suggestedEndpointFile(id, method, p),
				ExternalDocsURL: extDocs,
			})
		}
	}

	result.OperationCount = len(matched)
	if len(matched) > maxOps {
		result.TruncatedOperations = len(matched) - maxOps
		matched = matched[:maxOps]
		result.Notes = append(result.Notes, fmt.Sprintf(
			"Returned first %d of %d matching operations; narrow with path_prefix/tag or raise max_operations.",
			maxOps, result.OperationCount,
		))
	}
	result.Operations = matched

	for _, op := range matched {
		g := op.SuggestedGroup
		if g == "" {
			g = "misc"
		}
		result.SuggestedGroups[g] = append(result.SuggestedGroups[g], op.ID)
	}

	return result, nil
}

func openAPIServers(doc map[string]interface{}, format string) []string {
	var out []string
	if format == specFormatOpenAPI3 {
		if servers, ok := doc["servers"].([]interface{}); ok {
			for _, s := range servers {
				sm, _ := s.(map[string]interface{})
				if u := strField(sm, "url"); u != "" {
					out = append(out, u)
				}
			}
		}

		return out
	}

	host := strField(doc, "host")
	basePath := strField(doc, "basePath")
	schemes := stringSlice(doc["schemes"])
	if host == "" {
		return out
	}
	if len(schemes) == 0 {
		schemes = []string{"https"}
	}
	for _, sch := range schemes {
		out = append(out, strings.TrimRight(sch+"://"+host+basePath, "/"))
	}

	return out
}

func openAPISecuritySchemes(doc map[string]interface{}, format string) []SpecSecurity {
	var schemes map[string]interface{}
	if format == specFormatOpenAPI3 {
		if comp, ok := doc["components"].(map[string]interface{}); ok {
			schemes, _ = comp["securitySchemes"].(map[string]interface{})
		}
	} else {
		schemes, _ = doc["securityDefinitions"].(map[string]interface{})
	}
	if schemes == nil {
		return nil
	}

	names := sortedKeys(schemes)
	out := make([]SpecSecurity, 0, len(names))
	for _, name := range names {
		sm, _ := schemes[name].(map[string]interface{})
		out = append(out, SpecSecurity{
			Name:         name,
			Type:         strField(sm, "type"),
			Scheme:       strField(sm, "scheme"),
			In:           strField(sm, "in"),
			BearerFormat: strField(sm, "bearerFormat"),
			Description:  truncateStr(strField(sm, "description"), 300),
		})
	}

	return out
}

func mergeParams(pathParams, opParams interface{}) []SpecParam {
	var raw []interface{}
	if a, ok := pathParams.([]interface{}); ok {
		raw = append(raw, a...)
	}
	if a, ok := opParams.([]interface{}); ok {
		raw = append(raw, a...)
	}

	out := make([]SpecParam, 0, len(raw))
	seen := map[string]bool{}
	for _, p := range raw {
		pm, _ := p.(map[string]interface{})
		if pm == nil {
			continue
		}
		name := strField(pm, "name")
		in := strField(pm, "in")
		key := in + ":" + name
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true

		schemaType := ""
		if sch, ok := pm["schema"].(map[string]interface{}); ok {
			schemaType = strField(sch, "type")
		}
		if schemaType == "" {
			schemaType = strField(pm, "type")
		}

		out = append(out, SpecParam{
			Name:        name,
			In:          in,
			Required:    boolField(pm, "required"),
			Description: truncateStr(strField(pm, "description"), 200),
			SchemaType:  schemaType,
		})
	}

	return out
}

func summarizeRequestBody(rb interface{}, format string) string {
	if format == specFormatOpenAPI2 {
		return "" // body params are in parameters for OAS2
	}
	body, _ := rb.(map[string]interface{})
	if body == nil {
		return ""
	}
	parts := []string{}
	if boolField(body, "required") {
		parts = append(parts, "required")
	}
	if content, ok := body["content"].(map[string]interface{}); ok {
		parts = append(parts, "content-types: "+strings.Join(sortedKeys(content), ", "))
	}
	if d := strField(body, "description"); d != "" {
		parts = append(parts, truncateStr(d, 200))
	}

	return strings.Join(parts, "; ")
}

func summarizeResponses(resp interface{}) map[string]string {
	rm, _ := resp.(map[string]interface{})
	if rm == nil {
		return nil
	}
	out := map[string]string{}
	for _, code := range sortedKeys(rm) {
		entry, _ := rm[code].(map[string]interface{})
		desc := strField(entry, "description")
		if desc == "" {
			desc = "(no description)"
		}
		out[code] = truncateStr(desc, 200)
	}

	return out
}

func summarizeSecurity(opSec, rootSec interface{}) []string {
	list, ok := opSec.([]interface{})
	if !ok || len(list) == 0 {
		list, _ = rootSec.([]interface{})
	}
	var names []string
	seen := map[string]bool{}
	for _, item := range list {
		m, _ := item.(map[string]interface{})
		for _, k := range sortedKeys(m) {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}

	return names
}

func parsePostman(doc map[string]interface{}, source, pathPrefix string, maxOps int) (*SpecIngestResult, error) {
	info, _ := doc["info"].(map[string]interface{})
	result := &SpecIngestResult{
		Format:          specFormatPostman,
		Source:          source,
		Title:           strField(info, "name"),
		Description:     truncateStr(strField(info, "description"), 500),
		SuggestedGroups: map[string][]string{},
		Notes: []string{
			"Parsed from Postman Collection. Request URLs may contain variables like {{baseUrl}}.",
		},
	}

	var matched []SpecOperation
	var walk func(items []interface{}, folder string)
	walk = func(items []interface{}, folder string) {
		for _, it := range items {
			im, _ := it.(map[string]interface{})
			if im == nil {
				continue
			}
			name := strField(im, "name")
			if nested, ok := im["item"].([]interface{}); ok {
				nextFolder := name
				if folder != "" && name != "" {
					nextFolder = folder + "/" + name
				} else if folder != "" {
					nextFolder = folder
				}
				walk(nested, nextFolder)

				continue
			}

			req, _ := im["request"].(map[string]interface{})
			if req == nil {
				// Sometimes request is a raw URL string — skip sparse entries.
				continue
			}

			method := strings.ToUpper(strField(req, "method"))
			if method == "" {
				method = "GET"
			}
			path := postmanPath(req)
			if pathPrefix != "" && !strings.Contains(path, pathPrefix) {
				continue
			}

			group := folder
			if group == "" {
				group = suggestedGroup(nil, path)
			} else {
				group = slugPath(strings.ReplaceAll(group, " ", "_"))
			}

			id := slugPath(name)
			if id == "" {
				id = strings.ToLower(method) + "_" + slugPath(path)
			}

			matched = append(matched, SpecOperation{
				ID:             id,
				Method:         method,
				Path:           path,
				Summary:        name,
				Description:    truncateStr(strField(im, "description"), 800),
				SuggestedGroup: group,
				SuggestedFile:  suggestedEndpointFile(id, method, path),
				Parameters:     postmanParams(req),
				RequestBody:    postmanBodySummary(req),
			})
		}
	}

	items, _ := doc["item"].([]interface{})
	walk(items, "")

	result.OperationCount = len(matched)
	if len(matched) > maxOps {
		result.TruncatedOperations = len(matched) - maxOps
		matched = matched[:maxOps]
		result.Notes = append(result.Notes, fmt.Sprintf(
			"Returned first %d of %d matching requests; narrow with path_prefix or raise max_operations.",
			maxOps, result.OperationCount,
		))
	}
	result.Operations = matched

	for _, op := range matched {
		g := op.SuggestedGroup
		if g == "" {
			g = "misc"
		}
		result.SuggestedGroups[g] = append(result.SuggestedGroups[g], op.ID)
	}

	return result, nil
}

func postmanPath(req map[string]interface{}) string {
	switch u := req["url"].(type) {
	case string:
		return u
	case map[string]interface{}:
		if raw := strField(u, "raw"); raw != "" {
			return raw
		}
		if pathParts, ok := u["path"].([]interface{}); ok {
			parts := make([]string, 0, len(pathParts))
			for _, p := range pathParts {
				parts = append(parts, fmt.Sprint(p))
			}

			return "/" + strings.Join(parts, "/")
		}
	}

	return ""
}

func postmanParams(req map[string]interface{}) []SpecParam {
	var out []SpecParam
	urlMap, _ := req["url"].(map[string]interface{})
	if urlMap != nil {
		if q, ok := urlMap["query"].([]interface{}); ok {
			for _, item := range q {
				im, _ := item.(map[string]interface{})
				if im == nil {
					continue
				}
				out = append(out, SpecParam{
					Name:        strField(im, "key"),
					In:          "query",
					Description: truncateStr(strField(im, "description"), 200),
				})
			}
		}
	}
	if headers, ok := req["header"].([]interface{}); ok {
		for _, item := range headers {
			im, _ := item.(map[string]interface{})
			if im == nil {
				continue
			}
			out = append(out, SpecParam{
				Name:        strField(im, "key"),
				In:          "header",
				Description: truncateStr(strField(im, "description"), 200),
			})
		}
	}

	return out
}

func postmanBodySummary(req map[string]interface{}) string {
	body, _ := req["body"].(map[string]interface{})
	if body == nil {
		return ""
	}
	mode := strField(body, "mode")
	if mode == "" {
		return ""
	}

	return "mode=" + mode
}

func suggestedGroup(tags []string, path string) string {
	if len(tags) > 0 && tags[0] != "" {
		return slugPath(tags[0])
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		if p == "" || strings.HasPrefix(p, "{") || strings.HasPrefix(p, ":") {
			continue
		}
		// Skip common version segments.
		if strings.HasPrefix(strings.ToLower(p), "v") && len(p) <= 3 {
			continue
		}

		return slugPath(p)
	}

	return "misc"
}

func suggestedEndpointFile(id, method, path string) string {
	base := slugPath(id)
	if base == "" {
		base = strings.ToLower(method) + "_" + slugPath(path)
	}

	return base + ".md"
}

func slugPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			// camelCase / PascalCase → snake_case
			if i > 0 && !lastUnderscore {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
			lastUnderscore = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	return strings.Trim(b.String(), "_")
}

func strField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if v == nil {
			return ""
		}

		return fmt.Sprint(v)
	}
}

func boolField(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key].(bool)

	return ok && v
}

func stringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}

	return out
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func containsFold(items []string, wantLower string) bool {
	for _, item := range items {
		if strings.ToLower(item) == wantLower {
			return true
		}
	}

	return false
}

func truncateStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}

	return s[:max] + "…"
}
