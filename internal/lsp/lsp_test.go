package lsp

import (
	"encoding/json"
	"testing"
)

func TestLSPError_Error(t *testing.T) {
	e := &LSPError{Code: -32000, Message: "server error"}
	if e.Error() != "LSP error -32000: server error" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestPosition_RoundTrip(t *testing.T) {
	pos := Position{Line: 10, Character: 5}
	data, _ := json.Marshal(pos)

	var parsed Position
	json.Unmarshal(data, &parsed)

	if parsed.Line != 10 || parsed.Character != 5 {
		t.Errorf("round-trip failed: %+v", parsed)
	}
}

func TestRange_RoundTrip(t *testing.T) {
	r := Range{
		Start: Position{Line: 1, Character: 2},
		End:   Position{Line: 3, Character: 4},
	}
	data, _ := json.Marshal(r)

	var parsed Range
	json.Unmarshal(data, &parsed)

	if parsed.Start.Line != 1 || parsed.End.Character != 4 {
		t.Errorf("round-trip failed: %+v", parsed)
	}
}

func TestLocation_RoundTrip(t *testing.T) {
	loc := Location{
		URI: "file:///test.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}
	data, _ := json.Marshal(loc)

	var parsed Location
	json.Unmarshal(data, &parsed)

	if parsed.URI != "file:///test.go" {
		t.Errorf("URI = %q", parsed.URI)
	}
}

func TestInitializeParams_Marshal(t *testing.T) {
	params := InitializeParams{
		ProcessID: nil,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCaps{
				Completion: &CompletionCaps{DynamicRegistration: false},
			},
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed InitializeParams
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Capabilities.TextDocument == nil {
		t.Error("TextDocument capabilities should not be nil")
	}
}

func TestCompletionParams_Marshal(t *testing.T) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///main.go"},
		Position:     Position{Line: 10, Character: 5},
	}

	data, _ := json.Marshal(params)

	var parsed CompletionParams
	json.Unmarshal(data, &parsed)

	if parsed.TextDocument.URI != "file:///main.go" {
		t.Errorf("URI = %q", parsed.TextDocument.URI)
	}
}

func TestCompletionItem_Marshal(t *testing.T) {
	items := []CompletionItem{
		{Label: "fmt.Println", Kind: 3, Detail: "Print with newline"},
		{Label: "fmt.Printf", Kind: 3, Detail: "Print formatted"},
	}

	data, _ := json.Marshal(items)

	var parsed []CompletionItem
	json.Unmarshal(data, &parsed)

	if len(parsed) != 2 {
		t.Fatalf("items count = %d, want 2", len(parsed))
	}
	if parsed[0].Label != "fmt.Println" {
		t.Errorf("label = %q", parsed[0].Label)
	}
}

func TestHover_Marshal(t *testing.T) {
	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "**Println** formats and writes to standard output.",
		},
	}

	data, _ := json.Marshal(hover)

	var parsed Hover
	json.Unmarshal(data, &parsed)

	if parsed.Contents.Kind != "markdown" {
		t.Errorf("Kind = %q", parsed.Contents.Kind)
	}
}

func TestDocumentSymbol_Marshal(t *testing.T) {
	symbols := []DocumentSymbol{
		{
			Name: "main",
			Kind: 12, // Function
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 10, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 4},
			},
			Children: []DocumentSymbol{
				{Name: "helper", Kind: 12},
			},
		},
	}

	data, _ := json.Marshal(symbols)

	var parsed []DocumentSymbol
	json.Unmarshal(data, &parsed)

	if len(parsed) != 1 {
		t.Fatalf("symbols count = %d", len(parsed))
	}
	if parsed[0].Name != "main" {
		t.Errorf("name = %q", parsed[0].Name)
	}
	if len(parsed[0].Children) != 1 {
		t.Errorf("children count = %d", len(parsed[0].Children))
	}
}

func TestInitializeResult_Marshal(t *testing.T) {
	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1,
			},
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		ServerInfo: ServerInfoResult{
			Name:    "gopls",
			Version: "0.15.0",
		},
	}

	data, _ := json.Marshal(result)

	var parsed InitializeResult
	json.Unmarshal(data, &parsed)

	if parsed.ServerInfo.Name != "gopls" {
		t.Errorf("ServerInfo.Name = %q", parsed.ServerInfo.Name)
	}
	if !parsed.Capabilities.HoverProvider {
		t.Error("HoverProvider should be true")
	}
}

func TestDidOpenParams_Marshal(t *testing.T) {
	params := DidOpenParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///main.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main\n\nfunc main() {}\n",
		},
	}

	data, _ := json.Marshal(params)

	var parsed DidOpenParams
	json.Unmarshal(data, &parsed)

	if parsed.TextDocument.LanguageID != "go" {
		t.Errorf("LanguageID = %q", parsed.TextDocument.LanguageID)
	}
}
