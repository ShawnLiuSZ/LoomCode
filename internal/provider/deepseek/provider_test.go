package deepseek

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
)

func TestCountTokens(t *testing.T) {
	path := filepath.Join("..", "..", "..", "doc", "deepseek_v3_tokenizer", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("tokenizer.json not found at %s: %v", path, err)
	}

	tok, err := loadTokenizer()
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}
	if tok == nil {
		t.Fatal("tokenizer is nil")
	}

	adapter := &Adapter{}
	p, err := adapter.Create(provider.Config{
		Name:    "deepseek",
		BaseURL: "https://api.deepseek.com",
		Models: []provider.ModelConfigItem{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
		},
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	tc, ok := p.(provider.TokenCounter)
	if !ok {
		t.Fatalf("DeepSeekProvider does not implement TokenCounter")
	}

	cases := []struct {
		text string
		want int
	}{
		{"hello", 1},
		{"hello world", 2},
		{"The quick brown fox jumps over the lazy dog.", 10},
	}
	for _, c := range cases {
		got, err := tc.CountTokens(c.text)
		if err != nil {
			t.Errorf("CountTokens(%q) error: %v", c.text, err)
			continue
		}
		if got != c.want {
			t.Errorf("CountTokens(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}
