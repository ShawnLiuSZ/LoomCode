package deepseek

import (
	"regexp"
	"strings"
)

// preTokenizer implements the LlamaTokenizerFast pretokenization pipeline.
type preTokenizer struct {
	splitters []*splitter
	byteLevel *byteLevel
}

type splitter struct {
	re     *regexp.Regexp
	invert bool
}

type byteLevel struct{}

func newPreTokenizer(cfg preTokenizerConfig) *preTokenizer {
	pt := &preTokenizer{}
	if cfg.Type != "Sequence" {
		return pt
	}
	for _, e := range cfg.Pretokenizers {
		switch e.Type {
		case "Split":
			if e.Behavior != "Isolated" {
				continue
			}
			re, err := regexp.Compile(e.Pattern.Regex)
			if err != nil {
				// Go regexp does not support negative lookahead. Try to strip
				// the trailing whitespace alternatives and recompile.
				cleaned := stripLookahead(e.Pattern.Regex)
				re, err = regexp.Compile(cleaned)
				if err != nil {
					continue
				}
			}
			pt.splitters = append(pt.splitters, &splitter{re: re, invert: e.Invert})
		case "ByteLevel":
			pt.byteLevel = &byteLevel{}
		}
	}
	return pt
}

// stripLookahead removes unsupported lookahead and trailing whitespace alternatives
// from the DeepSeek pretokenizer regex while keeping the core token patterns.
func stripLookahead(re string) string {
	// Drop the trailing "|\s+(?!\S)|\s+" group which is only for trailing whitespace.
	idx := strings.LastIndex(re, "|\\s+(?!\\S)|\\s+")
	if idx >= 0 {
		return re[:idx]
	}
	idx = strings.LastIndex(re, "|\\s+")
	if idx >= 0 {
		return re[:idx]
	}
	return re
}

// split runs the pretokenizer pipeline on a piece of text.
func (pt *preTokenizer) split(text string) []string {
	if text == "" {
		return nil
	}

	pieces := []string{text}
	for _, sp := range pt.splitters {
		pieces = sp.apply(pieces)
	}

	if pt.byteLevel != nil {
		var out []string
		for _, p := range pieces {
			out = append(out, pt.byteLevel.apply(p)...)
		}
		pieces = out
	}
	return pieces
}

// apply splits each input piece according to the regex.
// With invert=false, the matched substrings are kept as separate tokens.
func (s *splitter) apply(pieces []string) []string {
	var out []string
	for _, p := range pieces {
		if p == "" {
			continue
		}
		matches := s.re.FindAllStringIndex(p, -1)
		if len(matches) == 0 {
			out = append(out, p)
			continue
		}
		last := 0
		for _, m := range matches {
			if s.invert {
				if m[0] > last {
					out = append(out, p[last:m[0]])
				}
				out = append(out, p[m[0]:m[1]])
			} else {
				if m[0] > last {
					out = append(out, p[last:m[0]])
				}
				out = append(out, p[m[0]:m[1]])
			}
			last = m[1]
		}
		if last < len(p) {
			out = append(out, p[last:])
		}
	}
	return out
}

// apply converts a piece into byte-level representation used by the BPE vocab.
// The vocab stores bytes as their Unicode code point (e.g. byte 0x20 -> "Ġ").
func (bl *byteLevel) apply(piece string) []string {
	if piece == "" {
		return nil
	}
	// ByteLevel represents each byte as a specific Unicode char.
	// For ASCII printable (32-126) and some control bytes, HuggingFace maps
	// them to a set of glyphs. We use the standard GPT-2/LLaMA byte decoder.
	runes := make([]rune, 0, len(piece))
	for i := 0; i < len(piece); i++ {
		runes = append(runes, byteToUnicode(piece[i]))
	}
	// The encoded string forms a single word token for BPE.
	return []string{string(runes)}
}

// byteToUnicodeMap generates the GPT-2 byte-level BPE mapping used by LLaMA/DeepSeek.
// It matches HuggingFace/transformers bytes_to_unicode().
func byteToUnicodeMap() [256]rune {
	var table [256]rune
	bs := make([]int, 0, 256)
	// Printable ASCII range 33-126.
	for i := '!'; i <= '~'; i++ {
		bs = append(bs, int(i))
	}
	// Latin-1 ranges 161-172 and 174-255.
	for i := '¡'; i <= '¬'; i++ {
		bs = append(bs, int(i))
	}
	for i := '®'; i <= 'ÿ'; i++ {
		bs = append(bs, int(i))
	}
	cs := make([]int, len(bs))
	copy(cs, bs)
	n := 0
	for b := 0; b < 256; b++ {
		found := false
		for _, v := range bs {
			if v == b {
				found = true
				break
			}
		}
		if !found {
			bs = append(bs, b)
			cs = append(cs, 256+n)
			n++
		}
	}
	for i, b := range bs {
		table[b] = rune(cs[i])
	}
	return table
}

var _byteToUnicode = byteToUnicodeMap()

// byteToUnicode maps a byte to its ByteLevel Unicode representation.
func byteToUnicode(b byte) rune { return _byteToUnicode[b] }

