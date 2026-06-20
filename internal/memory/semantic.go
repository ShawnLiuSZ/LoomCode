package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
)

// EmbeddingProvider 嵌入提供者接口
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// Document 文档
type Document struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Vector   []float64         `json:"-"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Document *Document `json:"document"`
	Score    float64   `json:"score"`
}

// SemanticIndex 语义索引
type SemanticIndex struct {
	mu          sync.RWMutex
	documents   map[string]*Document
	embeddings  EmbeddingProvider
	dimensions  int
	cacheMu     sync.RWMutex        // 保护 embedCache
	embedCache  map[string][]float64 // content hash → cached embedding
}

// NewSemanticIndex 创建语义索引
func NewSemanticIndex(embeddings EmbeddingProvider, dimensions int) *SemanticIndex {
	if dimensions <= 0 {
		dimensions = 384 // 默认维度
	}
	return &SemanticIndex{
		documents:  make(map[string]*Document),
		embeddings: embeddings,
		dimensions: dimensions,
		embedCache: make(map[string][]float64),
	}
}

// contentHash 计算内容的哈希用于缓存键
func contentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", h[:16])
}

// getEmbed 获取嵌入向量（带缓存）
func (idx *SemanticIndex) getEmbed(ctx context.Context, text string) ([]float64, error) {
	hash := contentHash(text)

	idx.cacheMu.RLock()
	if cached, ok := idx.embedCache[hash]; ok {
		idx.cacheMu.RUnlock()
		return cached, nil
	}
	idx.cacheMu.RUnlock()

	vector, err := idx.embeddings.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	idx.cacheMu.Lock()
	idx.embedCache[hash] = vector
	idx.cacheMu.Unlock()

	return vector, nil
}

// Add 添加文档
func (idx *SemanticIndex) Add(ctx context.Context, doc *Document) error {
	if doc.ID == "" {
		return fmt.Errorf("document ID is required")
	}

	vector, err := idx.getEmbed(ctx, doc.Content)
	if err != nil {
		return fmt.Errorf("embed document: %w", err)
	}

	doc.Vector = normalizeVector(vector)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.documents[doc.ID] = doc
	return nil
}

// AddBatch 批量添加文档（使用 EmbedBatch 减少 HTTP 调用）
func (idx *SemanticIndex) AddBatch(ctx context.Context, docs []*Document) error {
	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	// 检查缓存，只 embed 未缓存的
	toEmbed := make([]int, 0)
	uncachedTexts := make([]string, 0)

	idx.cacheMu.RLock()
	for i, text := range texts {
		hash := contentHash(text)
		if _, ok := idx.embedCache[hash]; !ok {
			toEmbed = append(toEmbed, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}
	idx.cacheMu.RUnlock()

	// 批量 embed 未缓存的
	if len(uncachedTexts) > 0 {
		vectors, err := idx.embeddings.EmbedBatch(ctx, uncachedTexts)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}
		idx.cacheMu.Lock()
		for j, i := range toEmbed {
			hash := contentHash(texts[i])
			idx.embedCache[hash] = vectors[j]
		}
		idx.cacheMu.Unlock()
	}

	// 分配向量
	idx.cacheMu.RLock()
	for i, doc := range docs {
		hash := contentHash(texts[i])
		doc.Vector = normalizeVector(idx.embedCache[hash])
	}
	idx.cacheMu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, doc := range docs {
		idx.documents[doc.ID] = doc
	}
	return nil
}

// Get 获取文档
func (idx *SemanticIndex) Get(id string) (*Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	doc, ok := idx.documents[id]
	return doc, ok
}

// Delete 删除文档
func (idx *SemanticIndex) Delete(id string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.documents[id]; ok {
		delete(idx.documents, id)
		return true
	}
	return false
}

// Search 搜索相似文档
func (idx *SemanticIndex) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	queryVector, err := idx.getEmbed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	queryVector = normalizeVector(queryVector)

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SearchResult
	for _, doc := range idx.documents {
		if doc.Vector == nil {
			continue
		}
		score := dotProduct(queryVector, doc.Vector)
		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 返回前 topK 个结果
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// Size 返回文档数量
func (idx *SemanticIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.documents)
}

// Clear 清除所有文档和缓存
func (idx *SemanticIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.documents = make(map[string]*Document)

	idx.cacheMu.Lock()
	defer idx.cacheMu.Unlock()
	idx.embedCache = make(map[string][]float64)
}

// Save 保存索引到文件
func (idx *SemanticIndex) Save(path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	data, err := json.MarshalIndent(idx.documents, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load 从文件加载索引
func (idx *SemanticIndex) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	return json.Unmarshal(data, &idx.documents)
}

// normalizeVector 归一化向量（L2 norm = 1）
func normalizeVector(v []float64) []float64 {
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	norm = math.Sqrt(norm)
	result := make([]float64, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
}

// dotProduct 计算点积（预归一化向量的余弦相似度）
func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// MockEmbeddings 模拟嵌入提供者（用于测试）
type MockEmbeddings struct {
	dimensions int
}

// NewMockEmbeddings 创建模拟嵌入提供者
func NewMockEmbeddings(dimensions int) *MockEmbeddings {
	return &MockEmbeddings{dimensions: dimensions}
}

// Embed 生成模拟嵌入向量
func (m *MockEmbeddings) Embed(ctx context.Context, text string) ([]float64, error) {
	vector := make([]float64, m.dimensions)
	hash := simpleHash(text)
	for i := range vector {
		vector[i] = math.Sin(hash+float64(i)*31) * 0.5
	}
	return vector, nil
}

// EmbedBatch 批量生成模拟嵌入向量
func (m *MockEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, text := range texts {
		vector, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result[i] = vector
	}
	return result, nil
}

// simpleHash 简单哈希函数
func simpleHash(s string) float64 {
	var h float64
	for _, c := range s {
		h = h*31 + float64(c)
	}
	return h
}

// IndexStats 索引统计
type IndexStats struct {
	Documents  int     `json:"documents"`
	Dimensions int     `json:"dimensions"`
	AvgLength  float64 `json:"avg_length"`
}

// Stats 获取索引统计
func (idx *SemanticIndex) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	stats := IndexStats{
		Documents:  len(idx.documents),
		Dimensions: idx.dimensions,
	}

	if len(idx.documents) > 0 {
		var totalLen int
		for _, doc := range idx.documents {
			totalLen += len(doc.Content)
		}
		stats.AvgLength = float64(totalLen) / float64(len(idx.documents))
	}

	return stats
}

// FilterByMetadata 按元数据过滤
func (idx *SemanticIndex) FilterByMetadata(key, value string) []*Document {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []*Document
	for _, doc := range idx.documents {
		if doc.Metadata != nil && doc.Metadata[key] == value {
			results = append(results, doc)
		}
	}
	return results
}

// SearchWithFilter 带过滤的搜索
func (idx *SemanticIndex) SearchWithFilter(ctx context.Context, query string, topK int, filterKey, filterValue string) ([]SearchResult, error) {
	queryVector, err := idx.getEmbed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	queryVector = normalizeVector(queryVector)

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SearchResult
	for _, doc := range idx.documents {
		if doc.Vector == nil {
			continue
		}

		if filterKey != "" && doc.Metadata != nil && doc.Metadata[filterKey] != filterValue {
			continue
		}

		score := dotProduct(queryVector, doc.Vector)
		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 返回前 topK 个结果
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// Highlight 高亮匹配的文本
func Highlight(text, query string) string {
	terms := strings.Fields(query)
	result := text

	for _, term := range terms {
		upper := strings.ToUpper(term)
		result = strings.ReplaceAll(result, term, "**"+term+"**")
		result = strings.ReplaceAll(result, upper, "**"+upper+"**")
	}

	return result
}
