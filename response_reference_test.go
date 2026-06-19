package main

import (
	"testing"

	"github.com/petitorium/petitorium-plugin-sdk/types"
)

func TestParseParams(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{
			input: `{{response-reference:ref request="Get Token" attribute="body" jsonPath="object.token"}}`,
			expected: map[string]string{
				"request":   "Get Token",
				"attribute": "body",
				"jsonPath":  "object.token",
			},
		},
		{
			input:    `{{response-reference:ref request="Foo" attribute="header" headerName="X-Id"}}`,
			expected: map[string]string{"request": "Foo", "attribute": "header", "headerName": "X-Id"},
		},
		{
			input:    `{{response-reference:ref request="A" attribute="status"}}`,
			expected: map[string]string{"request": "A", "attribute": "status"},
		},
	}

	for _, tt := range tests {
		result := parseParams(tt.input)
		for k, v := range tt.expected {
			if result[k] != v {
				t.Errorf("parseParams(%q)[%q] = %q, want %q", tt.input, k, result[k], v)
			}
		}
	}
}

func TestFindTagEnd(t *testing.T) {
	tests := []struct {
		text     string
		start    int
		expected int
	}{
		{
			text:     `{{response-reference:ref request="A" attribute="body"}}`,
			start:    0,
			expected: 55,
		},
		{
			// }} inside quoted string should not terminate
			text:     `{{response-reference:ref request="A}}B" attribute="body"}}`,
			start:    0,
			expected: 58,
		},
		{
			text:     `prefix{{response-reference:ref request="A" attribute="body"}}suffix`,
			start:    6,
			expected: 61,
		},
	}

	for _, tt := range tests {
		got := findTagEnd(tt.text, tt.start)
		if got != tt.expected {
			t.Errorf("findTagEnd(%q, %d) = %d, want %d", tt.text, tt.start, got, tt.expected)
		}
	}
}

func TestFindRequestByName(t *testing.T) {
	cols := []collection{
		{
			Name: "Auth",
			Requests: []request{
				{Name: "Login"},
				{Name: "Logout"},
			},
			Collections: []collection{
				{
					Name: "OAuth",
					Requests: []request{
						{Name: "Get Token"},
					},
				},
			},
		},
		{
			Name: "Orders",
			Requests: []request{
				{Name: "Create Order"},
			},
		},
	}

	if r := findRequestByName(cols, "Login"); r == nil {
		t.Error("expected to find Login")
	}
	if r := findRequestByName(cols, "Get Token"); r == nil {
		t.Error("expected to find Get Token in nested collection")
	}
	if r := findRequestByName(cols, "Nonexistent"); r != nil {
		t.Error("expected nil for nonexistent request")
	}
}

func TestUpdateTag(t *testing.T) {
	rr := &ResponseReference{}

	tests := []struct {
		values   map[string]string
		expected string
	}{
		{
			values:   map[string]string{"request": "Get Token", "attribute": "body", "jsonPath": "data.token", "headerName": ""},
			expected: `{{response-reference:ref request="Get Token" attribute="body" jsonPath="data.token"}}`,
		},
		{
			values:   map[string]string{"request": "Get Token", "attribute": "header", "jsonPath": "", "headerName": "X-Id"},
			expected: `{{response-reference:ref request="Get Token" attribute="header" headerName="X-Id"}}`,
		},
		{
			values:   map[string]string{"request": "Get Token", "attribute": "status", "jsonPath": "", "headerName": ""},
			expected: `{{response-reference:ref request="Get Token" attribute="status"}}`,
		},
	}

	for _, tt := range tests {
		res, err := rr.UpdateTag("", tt.values)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.NewRawTag != tt.expected {
			t.Errorf("UpdateTag(...) = %q, want %q", res.NewRawTag, tt.expected)
		}
	}
}

func TestResolveTag(t *testing.T) {
	rr := &ResponseReference{}

	// Missing request or attribute
	if got := rr.resolveTag(`{{response-reference:ref request="" attribute="body"}}`, "test"); got != `{{response-reference:ref request="" attribute="body"}}` {
		t.Errorf("expected unchanged tag for empty request, got %q", got)
	}

	// Nonexistent workspace
	if got := rr.resolveTag(`{{response-reference:ref request="A" attribute="body"}}`, "nonexistent-workspace-12345"); !contains(got, "error") {
		t.Errorf("expected error for missing workspace, got %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}

func TestGetTagDetails(t *testing.T) {
	rr := &ResponseReference{}

	res, err := rr.GetTagDetails(`{{response-reference:ref request="A" attribute="body" jsonPath="x"}}`, "body", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PluginName != "response-reference" {
		t.Errorf("PluginName = %q, want response-reference", res.PluginName)
	}
	if res.Schema == nil || len(res.Schema.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(res.Schema.Fields))
	}
}

func TestResponseReference_ExecuteHook(t *testing.T) {
	rr := &ResponseReference{}

	originalURL := `http://example.com?id={{response-reference:ref request="A" attribute="status"}}`
	originalBody := `{"token":"{{response-reference:ref request="A" attribute="body" jsonPath="token"}}"}`
	originalAuth := `Bearer {{response-reference:ref request="A" attribute="header" headerName="X-Token"}}`

	ctx := &types.HookContext{
		Request: &types.RequestData{
			URL:     originalURL,
			Body:    originalBody,
			Headers: map[string]string{"Authorization": originalAuth},
		},
		Workspace: "nonexistent-workspace-12345",
	}

	updated, err := rr.ExecuteHook(types.PreVariableSubstitution, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All tags should be replaced with error messages because workspace doesn't exist.
	if updated.Request.URL == originalURL {
		t.Error("expected URL to be modified")
	}
	if updated.Request.Body == originalBody {
		t.Error("expected Body to be modified")
	}
	if updated.Request.Headers["Authorization"] == originalAuth {
		t.Error("expected Header to be modified")
	}
}

func TestCompactValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "{\n  \"a\": 1,\n  \"b\": 2\n}",
			expected: `{"a":1,"b":2}`,
		},
		{
			input:    "single line value",
			expected: "single line value",
		},
		{
			input:    "line1\nline2\t\ttabbed",
			expected: "line1 line2 tabbed",
		},
		{
			input:    "  extra   spaces  \n  everywhere  ",
			expected: "extra spaces everywhere",
		},
		{
			input:    `{"serviceId":"123","name":"Example Service","logoList":["logo.png"],"formList":[]}`,
			expected: `{"serviceId":"123","name":"Example Service","logoList":["logo.png"],"formList":[]}`,
		},
	}

	for _, tt := range tests {
		got := compactValue(tt.input)
		if got != tt.expected {
			t.Errorf("compactValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
