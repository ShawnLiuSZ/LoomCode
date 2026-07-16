package deepseek

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenizerLoad(t *testing.T) {
	path := filepath.Join("..", "..", "..", "doc", "deepseek_v3_tokenizer", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("tokenizer.json not found at %s: %v", path, err)
	}
	tok, err := NewTokenizerFromFile(path)
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}
	if got := tok.VocabSize(); got < 100000 {
		t.Fatalf("vocab too small: %d", got)
	}
}

func TestTokenizerSimple(t *testing.T) {
	path := filepath.Join("..", "..", "..", "doc", "deepseek_v3_tokenizer", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("tokenizer.json not found at %s: %v", path, err)
	}
	tok, err := NewTokenizerFromFile(path)
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}

	cases := []struct {
		text string
		want []int
	}{
		{"hello", []int{33310}},
		{"hello world", []int{33310, 2058}},
		{"Hello, 世界!", []int{19923, 14, 223, 3427, 3}},
		{"12345", []int{6895, 1883}},
		{"The quick brown fox jumps over the lazy dog.", []int{671, 4787, 13769, 46012, 54994, 1060, 270, 41638, 6397, 16}},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			got := tok.Encode(c.text)
			if !sliceEqual(got, c.want) {
				t.Errorf("%q\n got: %v\nwant: %v", c.text, got, c.want)
			}
		})
	}
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTokenizerSpecialTokens(t *testing.T) {
	path := filepath.Join("..", "..", "..", "doc", "deepseek_v3_tokenizer", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("tokenizer.json not found at %s: %v", path, err)
	}
	tok, err := NewTokenizerFromFile(path)
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}

	text := "<｜begin▁of▁sentence｜>hello<｜end▁of▁sentence｜>"
	ids := tok.Encode(text)
	if len(ids) < 3 {
		t.Fatalf("expected at least 3 tokens, got %d: %v", len(ids), ids)
	}
	if ids[0] != 0 {
		t.Errorf("expected first token id 0 (bos), got %d", ids[0])
	}
	if ids[len(ids)-1] != 1 {
		t.Errorf("expected last token id 1 (eos), got %d", ids[len(ids)-1])
	}
}

func TestByteToUnicode(t *testing.T) {
	if byteToUnicode(' ') != '\u0120' {
		t.Errorf("space should map to U+0120, got %U", byteToUnicode(' '))
	}
	if byteToUnicode('A') != 'A' {
		t.Errorf("A should map to A, got %U", byteToUnicode('A'))
	}
	if byteToUnicode(0x7F) != '\u0121' {
		t.Errorf("0x7F should map to U+0121, got %U", byteToUnicode(0x7F))
	}
}

func BenchmarkTokenizer(b *testing.B) {
	path := filepath.Join("..", "..", "..", "doc", "deepseek_v3_tokenizer", "tokenizer.json")
	tok, err := NewTokenizerFromFile(path)
	if err != nil {
		b.Fatalf("load tokenizer: %v", err)
	}
	text := "The quick brown fox jumps over the lazy dog. " + fmt.Sprint(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tok.Count(text)
	}
}
