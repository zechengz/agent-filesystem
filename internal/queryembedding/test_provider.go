package queryembedding

import (
	"context"
	"hash/fnv"
	"strings"
	"unicode"
)

const TestDimension = 128

type TestProvider struct {
	model string
}

func NewTestProvider(model string) *TestProvider {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "test-embedding"
	}
	return &TestProvider{model: model}
}

func (p *TestProvider) Name() string {
	return "test"
}

func (p *TestProvider) Model() string {
	return p.model
}

func (p *TestProvider) Dimension() int {
	return TestDimension
}

func (p *TestProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

func (p *TestProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, testEmbed(text))
	}
	return out, nil
}

func testEmbed(text string) []float32 {
	vec := make([]float32, TestDimension)
	tokens := testTokens(text)
	if len(tokens) == 0 {
		return vec
	}
	for i, token := range tokens {
		testAddFeature(vec, token, 1)
		for _, alias := range testAliases[token] {
			testAddFeature(vec, alias, 0.55)
		}
		if i > 0 {
			testAddFeature(vec, tokens[i-1]+" "+token, 0.8)
		}
	}
	return normalize(vec)
}

func testTokens(text string) []string {
	all := make([]string, 0)
	tokens := make([]string, 0)
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		token := strings.ToLower(b.String())
		b.Reset()
		all = append(all, token)
		if testStopWords[token] {
			return
		}
		tokens = append(tokens, token)
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	if len(tokens) == 0 {
		return all
	}
	return tokens
}

func testAddFeature(vec []float32, feature string, weight float32) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(feature))
	sum := h.Sum64()
	idx := int(sum % uint64(len(vec)))
	if (sum>>63)&1 == 1 {
		weight = -weight
	}
	vec[idx] += weight
}

var testStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "do": true, "does": true, "for": true, "from": true,
	"how": true, "i": true, "in": true, "is": true, "it": true, "of": true,
	"on": true, "or": true, "the": true, "this": true, "to": true, "what": true,
	"when": true, "where": true, "why": true, "with": true,
}

var testAliases = map[string][]string{
	"auth":        {"login", "token", "tenant", "identity"},
	"checkpoint":  {"snapshot", "savepoint", "version", "restore"},
	"checkpoints": {"snapshot", "savepoint", "version", "restore"},
	"restore":     {"checkpoint", "snapshot", "recover"},
	"save":        {"checkpoint", "snapshot", "persist"},
	"savepoint":   {"checkpoint", "snapshot", "version"},
	"snapshot":    {"checkpoint", "savepoint", "version"},
	"snapshots":   {"checkpoint", "savepoint", "version"},
	"workspace":   {"project", "filesystem", "state"},
}
