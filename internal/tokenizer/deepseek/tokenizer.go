// Package deepseek provides a pure-Go DeepSeek V3 tokenizer for offline token counting.
package deepseek

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

//go:embed tokenizer.json
var defaultTokenizerJSON []byte

// Tokenizer is a pure-Go DeepSeek V3 tokenizer.
type Tokenizer struct {
	vocab        map[string]int
	merges       map[string]int
	specialTokens map[string]int
	preTokenizer   *preTokenizer
}

// tokenizerJSON is the subset of tokenizer.json we need.
type tokenizerJSON struct {
	AddedTokens []struct {
		Content string `json:"content"`
		ID      int    `json:"id"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
	PreTokenizer preTokenizerConfig `json:"pre_tokenizer"`
	Model        struct {
		Type   string         `json:"type"`
		Vocab  map[string]int `json:"vocab"`
		Merges []string       `json:"merges"`
	} `json:"model"`
}

type preTokenizerConfig struct {
	Type         string                `json:"type"`
	Pretokenizers []pretokenizerEntry  `json:"pretokenizers"`
}

type pretokenizerEntry struct {
	Type    string         `json:"type"`
	Pattern patternConfig  `json:"pattern,omitempty"`
	Behavior string        `json:"behavior,omitempty"`
	Invert  bool           `json:"invert,omitempty"`
	AddPrefixSpace bool    `json:"add_prefix_space,omitempty"`
	TrimOffsets    bool    `json:"trim_offsets,omitempty"`
	UseRegex       bool    `json:"use_regex,omitempty"`
}

type patternConfig struct {
	Regex string `json:"Regex"`
}

// NewDefaultTokenizer loads the embedded DeepSeek V3 tokenizer.
func NewDefaultTokenizer() (*Tokenizer, error) {
	return NewTokenizerFromBytes(defaultTokenizerJSON)
}

// NewTokenizerFromFile loads a tokenizer from a HuggingFace tokenizer.json file.
func NewTokenizerFromFile(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer file: %w", err)
	}
	return NewTokenizerFromBytes(data)
}

// NewTokenizerFromBytes loads a tokenizer from raw tokenizer.json bytes.
func NewTokenizerFromBytes(data []byte) (*Tokenizer, error) {
	var raw tokenizerJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse tokenizer json: %w", err)
	}
	if raw.Model.Type != "BPE" {
		return nil, fmt.Errorf("unsupported model type: %s", raw.Model.Type)
	}

	t := &Tokenizer{
		vocab:         make(map[string]int, len(raw.Model.Vocab)),
		merges:        make(map[string]int, len(raw.Model.Merges)),
		specialTokens: make(map[string]int),
	}
	for k, v := range raw.Model.Vocab {
		t.vocab[k] = v
	}
	for i, m := range raw.Model.Merges {
		parts := strings.Split(m, " ")
		if len(parts) != 2 {
			continue
		}
		t.merges[parts[0]+" "+parts[1]] = i
	}
	for _, st := range raw.AddedTokens {
		t.specialTokens[st.Content] = st.ID
		if _, ok := t.vocab[st.Content]; !ok {
			t.vocab[st.Content] = st.ID
		}
	}
	t.preTokenizer = newPreTokenizer(raw.PreTokenizer)
	return t, nil
}

// VocabSize returns the size of the vocabulary.
func (t *Tokenizer) VocabSize() int { return len(t.vocab) }

// Encode counts tokens in the input text.
func (t *Tokenizer) Encode(text string) []int {
	if text == "" {
		return nil
	}

	// Split special tokens first so they are not touched by BPE.
	pieces := t.splitSpecialTokens(text)
	var ids []int
	for _, piece := range pieces {
		if id, ok := t.specialTokens[piece]; ok {
			ids = append(ids, id)
			continue
		}
		for _, word := range t.preTokenizer.split(piece) {
			ids = append(ids, t.bpeEncode(word)...)
		}
	}
	return ids
}

// Count returns the number of tokens.
func (t *Tokenizer) Count(text string) int { return len(t.Encode(text)) }

// splitSpecialTokens splits text around special tokens, keeping them intact.
func (t *Tokenizer) splitSpecialTokens(text string) []string {
	if len(t.specialTokens) == 0 {
		return []string{text}
	}

	// Build a regex that matches any special token.
	var escaped []string
	for tok := range t.specialTokens {
		escaped = append(escaped, regexp.QuoteMeta(tok))
	}
	pattern := "(" + strings.Join(escaped, "|") + ")"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []string{text}
	}

	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}

	var parts []string
	last := 0
	for _, m := range matches {
		if m[0] > last {
			parts = append(parts, text[last:m[0]])
		}
		parts = append(parts, text[m[0]:m[1]])
		last = m[1]
	}
	if last < len(text) {
		parts = append(parts, text[last:])
	}
	return parts
}

// bpeEncode encodes a single pre-tokenized word using BPE.
func (t *Tokenizer) bpeEncode(word string) []int {
	if id, ok := t.vocab[word]; ok {
		return []int{id}
	}

	// Convert word to initial list of bytes (GPT-2/4 style byte-level BPE).
	wordBytes := []byte(word)
	if len(wordBytes) == 0 {
		return nil
	}

	// Initial tokens are individual UTF-8 bytes mapped to their vocab ids.
	tokens := make([]string, len(wordBytes))
	for i, b := range wordBytes {
		tokens[i] = string(rune(b))
	}

	for len(tokens) > 1 {
		bestRank := -1
		bestIdx := -1
		for i := 0; i < len(tokens)-1; i++ {
			pair := tokens[i] + " " + tokens[i+1]
			if rank, ok := t.merges[pair]; ok {
				if bestRank == -1 || rank < bestRank {
					bestRank = rank
					bestIdx = i
				}
			}
		}
		if bestIdx == -1 {
			break
		}
		merged := tokens[bestIdx] + tokens[bestIdx+1]
		tokens = append(tokens[:bestIdx], append([]string{merged}, tokens[bestIdx+2:]...)...)
	}

	ids := make([]int, 0, len(tokens))
	for _, tok := range tokens {
		if id, ok := t.vocab[tok]; ok {
			ids = append(ids, id)
		} else {
			// Fallback: encode each byte individually.
			for _, b := range []byte(tok) {
				if id, ok := t.vocab[string(rune(b))]; ok {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}
