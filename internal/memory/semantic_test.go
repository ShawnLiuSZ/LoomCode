package memory

import (
	"context"
	"testing"
)

func TestSemanticIndex(t *testing.T) {
	t.Run("NewSemanticIndex", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)
		if idx == nil {
			t.Fatal("expected non-nil index")
		}
		if idx.Size() != 0 {
			t.Errorf("expected size 0, got %d", idx.Size())
		}
	})

	t.Run("Add", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		doc := &Document{
			ID:      "1",
			Content: "test document",
		}

		err := idx.Add(context.Background(), doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if idx.Size() != 1 {
			t.Errorf("expected size 1, got %d", idx.Size())
		}
	})

	t.Run("Add_NoID", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		doc := &Document{
			Content: "test document",
		}

		err := idx.Add(context.Background(), doc)
		if err == nil {
			t.Error("expected error for document without ID")
		}
	})

	t.Run("Get", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		doc := &Document{ID: "1", Content: "test"}
		idx.Add(context.Background(), doc)

		got, ok := idx.Get("1")
		if !ok {
			t.Error("expected to find document")
		}
		if got.Content != "test" {
			t.Errorf("expected 'test', got %q", got.Content)
		}
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		_, ok := idx.Get("nonexistent")
		if ok {
			t.Error("expected not to find document")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		doc := &Document{ID: "1", Content: "test"}
		idx.Add(context.Background(), doc)

		ok := idx.Delete("1")
		if !ok {
			t.Error("expected delete to return true")
		}
		if idx.Size() != 0 {
			t.Errorf("expected size 0, got %d", idx.Size())
		}
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		ok := idx.Delete("nonexistent")
		if ok {
			t.Error("expected delete to return false")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		idx.Add(context.Background(), &Document{ID: "1", Content: "test1"})
		idx.Add(context.Background(), &Document{ID: "2", Content: "test2"})

		idx.Clear()

		if idx.Size() != 0 {
			t.Errorf("expected size 0, got %d", idx.Size())
		}
	})
}

func TestSemanticIndexSearch(t *testing.T) {
	t.Run("Search", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		idx.Add(context.Background(), &Document{ID: "1", Content: "hello world"})
		idx.Add(context.Background(), &Document{ID: "2", Content: "foo bar"})
		idx.Add(context.Background(), &Document{ID: "3", Content: "hello foo"})

		results, err := idx.Search(context.Background(), "hello", 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("Search_Empty", func(t *testing.T) {
		embeddings := NewMockEmbeddings(384)
		idx := NewSemanticIndex(embeddings, 384)

		results, err := idx.Search(context.Background(), "query", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestSemanticIndexAddBatch(t *testing.T) {
	embeddings := NewMockEmbeddings(384)
	idx := NewSemanticIndex(embeddings, 384)

	docs := []*Document{
		{ID: "1", Content: "doc1"},
		{ID: "2", Content: "doc2"},
		{ID: "3", Content: "doc3"},
	}

	err := idx.AddBatch(context.Background(), docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idx.Size() != 3 {
		t.Errorf("expected size 3, got %d", idx.Size())
	}
}

func TestSemanticIndexStats(t *testing.T) {
	embeddings := NewMockEmbeddings(384)
	idx := NewSemanticIndex(embeddings, 384)

	idx.Add(context.Background(), &Document{ID: "1", Content: "hello"})
	idx.Add(context.Background(), &Document{ID: "2", Content: "hello world"})

	stats := idx.Stats()
	if stats.Documents != 2 {
		t.Errorf("expected 2 documents, got %d", stats.Documents)
	}
	if stats.Dimensions != 384 {
		t.Errorf("expected 384 dimensions, got %d", stats.Dimensions)
	}
}

func TestSemanticIndexFilterByMetadata(t *testing.T) {
	embeddings := NewMockEmbeddings(384)
	idx := NewSemanticIndex(embeddings, 384)

	idx.Add(context.Background(), &Document{
		ID:       "1",
		Content:  "doc1",
		Metadata: map[string]string{"type": "code"},
	})
	idx.Add(context.Background(), &Document{
		ID:       "2",
		Content:  "doc2",
		Metadata: map[string]string{"type": "doc"},
	})

	results := idx.FilterByMetadata("type", "code")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			a:        []float64{1, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotProduct(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestHighlight(t *testing.T) {
	tests := []struct {
		text     string
		query    string
		expected string
	}{
		{
			text:     "hello world",
			query:    "hello",
			expected: "**hello** world",
		},
		{
			text:     "Hello World",
			query:    "Hello",
			expected: "**Hello** World",
		},
	}

	for _, tt := range tests {
		result := Highlight(tt.text, tt.query)
		if result != tt.expected {
			t.Errorf("Highlight(%q, %q) = %q, want %q", tt.text, tt.query, result, tt.expected)
		}
	}
}
