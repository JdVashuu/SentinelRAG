package engine

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/vashuu/sentinel-rag/internal/parser"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// VectorizedChunk combines a code chunk with its embedding
type VectorizedChunk struct {
	parser.CodeChunk
	Embedding []float32
}

// Embedder handles the conversion of text chunks into vectors
type Embedder struct {
	client *embeddings.EmbedderImpl
}

// NewOllamaEmbedder initializes an embedder using a local Ollama instance
func NewOllamaEmbedder(model string) (*Embedder, error) {
	llm, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ollama client: %w", err)
	}

	e, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return &Embedder{
		client: e,
	}, nil
}

// EmbedChunks takes a slice of CodeChunks and returns VectorizedChunks
func (e *Embedder) EmbedChunks(ctx context.Context, chunks []parser.CodeChunk) ([]VectorizedChunk, error) {
	tracer := otel.Tracer("engine")
	ctx, span := tracer.Start(ctx, "EmbedChunks")
	defer span.End()
	span.SetAttributes(attribute.Int("chunk_count", len(chunks)))

	if len(chunks) == 0 {
		return nil, nil
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// LangChainGo handles batching internally if the provider supports it
	vectors, err := e.client.EmbedDocuments(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to embed documents: %w", err)
	}

	if len(vectors) != len(chunks) {
		return nil, fmt.Errorf("mismatch between chunk count (%d) and vector count (%d)", len(chunks), len(vectors))
	}

	vectorized := make([]VectorizedChunk, len(chunks))
	for i := range chunks {
		vectorized[i] = VectorizedChunk{
			CodeChunk: chunks[i],
			Embedding: vectors[i],
		}
	}

	return vectorized, nil
}
