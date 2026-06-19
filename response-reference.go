// Package main provides the response-reference plugin for Petitorium.
// This plugin resolves template variables by extracting values from the
// response history of other requests in the same workspace.
//
// Tag syntax:
//
//	{{response-reference:ref request="Get Token" attribute="body" jsonPath="object.token"}}
//	{{response-reference:ref request="Get Token" attribute="header" headerName="X-Request-Id"}}
//	{{response-reference:ref request="Get Token" attribute="status"}}
//
// Supported attributes: body, header, status.
//
// To build: CGO_ENABLED=0 go build -o response-reference .
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-plugin"
	"github.com/mitchellh/go-homedir"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"

	"github.com/petitorium/petitorium-plugin-sdk/shared"
	"github.com/petitorium/petitorium-plugin-sdk/types"
)

// ResponseReference is a plugin that extracts values from other requests' responses.
type ResponseReference struct{}

// Name returns the plugin name.
func (rr *ResponseReference) Name() string {
	return "response-reference"
}

// Version returns the plugin version.
func (rr *ResponseReference) Version() string {
	return "1.0.0"
}

// Description returns the plugin description.
func (rr *ResponseReference) Description() string {
	return "Extracts values from other requests' response history using JSONPath or header names"
}

// Hooks returns the hook types this plugin implements.
func (rr *ResponseReference) Hooks() []types.HookType {
	return []types.HookType{types.PreVariableSubstitution}
}

// ExecuteHook executes a specific hook with the given context.
func (rr *ResponseReference) ExecuteHook(hookType types.HookType, ctx *types.HookContext) (*types.HookContext, error) {
	if hookType != types.PreVariableSubstitution {
		return ctx, nil
	}
	if ctx.Request == nil {
		return ctx, nil
	}

	ctx.Request.URL = rr.resolveTags(ctx.Request.URL, ctx.Workspace)
	ctx.Request.Body = rr.resolveTags(ctx.Request.Body, ctx.Workspace)

	for k, v := range ctx.Request.Headers {
		ctx.Request.Headers[k] = rr.resolveTags(v, ctx.Workspace)
	}

	return ctx, nil
}

var paramRegex = regexp.MustCompile(`(\w+)="([^"]*)"`)

// resolveTags finds all response-reference tags in text and replaces them.
func (rr *ResponseReference) resolveTags(text, workspace string) string {
	result := text
	for {
		idx := strings.Index(result, "{{response-reference:ref ")
		if idx == -1 {
			break
		}
		end := findTagEnd(result, idx)
		if end == -1 {
			break
		}

		tag := result[idx:end]
		resolved := rr.resolveTag(tag, workspace)
		result = result[:idx] + resolved + result[end:]
	}
	return result
}

// findTagEnd scans forward from start and returns the byte offset just after
// the matching "}}". It tracks quoted strings so }} inside values doesn't
// terminate prematurely.
func findTagEnd(text string, start int) int {
	i := start + len("{{response-reference:ref")
	inQuotes := false
	for i < len(text)-1 {
		if text[i] == '\\' && i+1 < len(text) && text[i+1] == '"' {
			i += 2
			continue
		}
		if text[i] == '"' {
			inQuotes = !inQuotes
			i++
			continue
		}
		if !inQuotes && text[i] == '}' && text[i+1] == '}' {
			return i + 2
		}
		i++
	}
	return -1
}

// resolveTag parses a single tag and returns the resolved value.
func (rr *ResponseReference) resolveTag(tag, workspace string) string {
	params := parseParams(tag)

	requestName := params["request"]
	attribute := params["attribute"]
	jsonPath := params["jsonPath"]
	headerName := params["headerName"]

	if requestName == "" || attribute == "" {
		return tag
	}

	collections, err := loadCollections(workspace)
	if err != nil {
		return fmt.Sprintf("[response-reference error: %v]", err)
	}

	req := findRequestByName(collections, requestName)
	if req == nil {
		return fmt.Sprintf("[response-reference error: request %q not found]", requestName)
	}

	if len(req.ResponseHistory) == 0 {
		return fmt.Sprintf("[response-reference error: no response history for %q]", requestName)
	}

	lastResp := req.ResponseHistory[len(req.ResponseHistory)-1]

	switch attribute {
	case "body":
		var value string
		if jsonPath == "" {
			value = lastResp.Body
		} else {
			result := gjson.Get(lastResp.Body, jsonPath)
			value = result.String()
		}
		return compactValue(value)
	case "header":
		if headerName == "" {
			return fmt.Sprintf("[response-reference error: headerName required for attribute=header]")
		}
		values, ok := lastResp.Headers[headerName]
		if !ok || len(values) == 0 {
			return fmt.Sprintf("[response-reference error: header %q not found]", headerName)
		}
		return values[0]
	case "status":
		return strconv.Itoa(lastResp.StatusCode)
	default:
		return fmt.Sprintf("[response-reference error: unknown attribute %q]", attribute)
	}
}

// parseParams extracts key="value" pairs from a raw tag string.
func parseParams(rawTag string) map[string]string {
	params := make(map[string]string)
	matches := paramRegex.FindAllStringSubmatch(rawTag, -1)
	for _, m := range matches {
		if len(m) == 3 {
			params[m[1]] = m[2]
		}
	}
	return params
}

// compactValue removes newlines, tabs, and unnecessary spaces from a value.
// If the input is valid JSON it is minified via json.Compact; otherwise
// insignificant whitespace is stripped and multiple spaces are collapsed.
func compactValue(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err == nil {
		return buf.String()
	}

	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// ============ Tag Editor Support ============

// GetTagDetails implements types.TagEditorCapable.
func (rr *ResponseReference) GetTagDetails(rawTag, context, workspace string) (*types.TagDetailsResponse, error) {
	params := parseParams(rawTag)

	requestName := params["request"]
	attribute := params["attribute"]
	jsonPath := params["jsonPath"]
	headerName := params["headerName"]

	if attribute == "" {
		attribute = "body"
	}

	// Build request dropdown options from workspace collections.
	requestOptions, requestLabels := buildRequestOptions(workspace)

	return &types.TagDetailsResponse{
		DisplayLabel: "Response Reference",
		PluginName:   "response-reference",
		Action:       "ref",
		Editable:     true,
		Schema: &types.TagEditorSchema{
			Fields: []types.TagField{
				{
					Key:          "request",
					Label:        "Request",
					FieldType:    "dropdown",
					Options:      requestOptions,
					OptionLabels: requestLabels,
					Required:     true,
					DefaultValue: requestName,
				},
				{
					Key:          "attribute",
					Label:        "Attribute",
					FieldType:    "dropdown",
					Options:      []string{"body", "header", "status"},
					OptionLabels: map[string]string{"body": "Body", "header": "Header", "status": "Status"},
					Required:     true,
					DefaultValue: attribute,
				},
				{
					Key:          "jsonPath",
					Label:        "JSONPath",
					FieldType:    "text",
					DefaultValue: jsonPath,
					DependsOn:    "attribute",
					DependsValue: "body",
					Disabled:     attribute != "body",
				},
				{
					Key:          "headerName",
					Label:        "Header Name",
					FieldType:    "text",
					DefaultValue: headerName,
					DependsOn:    "attribute",
					DependsValue: "header",
					Disabled:     attribute != "header",
				},
			},
		},
	}, nil
}

// UpdateTag implements types.TagEditorCapable.
func (rr *ResponseReference) UpdateTag(rawTag string, values map[string]string) (*types.UpdateTagResponse, error) {
	requestName := values["request"]
	attribute := values["attribute"]
	jsonPath := values["jsonPath"]
	headerName := values["headerName"]

	if attribute == "" {
		attribute = "body"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf(`request="%s"`, requestName))
	parts = append(parts, fmt.Sprintf(`attribute="%s"`, attribute))

	if attribute == "body" && jsonPath != "" {
		parts = append(parts, fmt.Sprintf(`jsonPath="%s"`, jsonPath))
	}
	if attribute == "header" && headerName != "" {
		parts = append(parts, fmt.Sprintf(`headerName="%s"`, headerName))
	}

	newTag := fmt.Sprintf("{{response-reference:ref %s}}", strings.Join(parts, " "))
	return &types.UpdateTagResponse{NewRawTag: newTag}, nil
}

// ============ Workspace / Collection Helpers ============

// Minimal YAML structs for loading collections without importing the main app.
type collection struct {
	Name        string       `yaml:"name"`
	Requests    []request    `yaml:"requests,omitempty"`
	Collections []collection `yaml:"collections,omitempty"`
}

type request struct {
	Name            string         `yaml:"name"`
	ResponseHistory []httpResponse `yaml:"response_history,omitempty"`
}

type httpResponse struct {
	StatusCode int                 `yaml:"status_code"`
	Headers    map[string][]string `yaml:"headers,omitempty"`
	Body       string              `yaml:"body,omitempty"`
}

// loadCollections reads the collections.yaml for the given workspace.
func loadCollections(workspace string) ([]collection, error) {
	if workspace == "" {
		return nil, fmt.Errorf("workspace name is empty")
	}
	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "petitorium", "workspaces", workspace, "collections.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read collections: %w", err)
	}

	var cols []collection
	if err := yaml.Unmarshal(data, &cols); err != nil {
		return nil, fmt.Errorf("failed to parse collections: %w", err)
	}
	return cols, nil
}

// findRequestByName searches all collections recursively for a request by name.
func findRequestByName(collections []collection, name string) *request {
	for i := range collections {
		for j := range collections[i].Requests {
			if collections[i].Requests[j].Name == name {
				return &collections[i].Requests[j]
			}
		}
		if found := findRequestByName(collections[i].Collections, name); found != nil {
			return found
		}
	}
	return nil
}

// buildRequestOptions walks all collections and builds dropdown options.
// The returned options are request names, and optionLabels map them to
// a display string like "Collection / Request Name".
func buildRequestOptions(workspace string) ([]string, map[string]string) {
	if workspace == "" {
		return nil, nil
	}
	collections, err := loadCollections(workspace)
	if err != nil {
		return nil, nil
	}

	options := make([]string, 0)
	labels := make(map[string]string)

	var walk func(cols []collection, parentPath string)
	walk = func(cols []collection, parentPath string) {
		for _, col := range cols {
			path := col.Name
			if parentPath != "" {
				path = parentPath + " / " + col.Name
			}
			for _, req := range col.Requests {
				options = append(options, req.Name)
				labels[req.Name] = path + " / " + req.Name
			}
			walk(col.Collections, path)
		}
	}

	walk(collections, "")
	return options, labels
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.Handshake,
		Plugins: map[string]plugin.Plugin{
			"response-reference": &shared.PetitoriumPlugin{Impl: &ResponseReference{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

// Compile-time interface checks.
var (
	_ types.Plugin           = &ResponseReference{}
	_ types.TagEditorCapable = &ResponseReference{}
)
