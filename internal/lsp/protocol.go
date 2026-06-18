package lsp

import "fmt"

// LSPError LSP 错误
type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *LSPError) Error() string {
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

// Position 位置
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range 范围
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location 位置
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier 文档标识
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier 带版本的文档标识
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentItem 文档内容
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentContentChangeEvent 内容变更
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

// InitializeParams initialize 参数
type InitializeParams struct {
	ProcessID    *int              `json:"processId"`
	RootURI      *string           `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities 客户端能力
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCaps `json:"textDocument,omitempty"`
}

// TextDocumentClientCaps 文档客户端能力
type TextDocumentClientCaps struct {
	Completion     *CompletionCaps     `json:"completion,omitempty"`
	Hover          *HoverCaps          `json:"hover,omitempty"`
	Definition     *DefinitionCaps     `json:"definition,omitempty"`
	DocumentSymbol *DocumentSymbolCaps `json:"documentSymbol,omitempty"`
	CodeAction     *CodeActionCaps     `json:"codeAction,omitempty"`
}

// CompletionCaps 补全能力
type CompletionCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// HoverCaps hover 能力
type HoverCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// DefinitionCaps 定义能力
type DefinitionCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// DocumentSymbolCaps 符号能力
type DocumentSymbolCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// CodeActionCaps 代码操作能力
type CodeActionCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// InitializeResult initialize 结果
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfoResult   `json:"serverInfo"`
}

// ServerCapabilities 服务器能力
type ServerCapabilities struct {
	TextDocumentSync *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	CompletionProvider *CompletionOptions     `json:"completionProvider,omitempty"`
	HoverProvider      bool                   `json:"hoverProvider"`
	DefinitionProvider bool                   `json:"definitionProvider"`
}

// TextDocumentSyncOptions 同步选项
type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"` // 1=full, 2=incremental
}

// CompletionOptions 补全选项
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// ServerInfoResult 服务器信息
type ServerInfoResult struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializedParams initialized 参数
type InitializedParams struct{}

// DidOpenParams didOpen 参数
type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeParams didChange 参数
type DidChangeParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// CompletionParams 补全参数
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// CompletionItem 补全项
type CompletionItem struct {
	Label      string `json:"label"`
	Kind       int    `json:"kind,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText string `json:"insertText,omitempty"`
}

// HoverParams hover 参数
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Hover hover 结果
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent 标记内容
type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown" | "plaintext"
	Value string `json:"value"`
}

// DefinitionParams 定义参数
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// DocumentSymbolParams 文档符号参数
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DocumentSymbol 文档符号
type DocumentSymbol struct {
	Name           string            `json:"name"`
	Kind           int               `json:"kind"`
	Range          Range             `json:"range"`
	SelectionRange Range             `json:"selectionRange"`
	Children       []DocumentSymbol  `json:"children,omitempty"`
}
