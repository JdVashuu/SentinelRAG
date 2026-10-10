package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"go.opentelemetry.io/otel"
)

// RewriteQuery uses the LLM to transform a vague user question into a search-optimized technical query.
// This is a "Self-Healing" pattern for retrieval.
func (g *Generator) RewriteQuery(ctx context.Context, question string) (string, error) {
	tracer := otel.Tracer("engine")
	ctx, span := tracer.Start(ctx, "RewriteQuery")
	defer span.End()

	systemPrompt := `You are a Search Query Optimizer.
Your goal is to rewrite the user's question into a technical search query that is more likely to find relevant code in a vector database.
- Use technical terms (e.g., 'initialization', 'concurrency', 'error handling').
- Focus on function names, types, and architectural patterns.
- Return ONLY the rewritten query text. No preamble.
- Use only english to describe the query.
`

	userPrompt := fmt.Sprintf("Original Question: %s\nRewritten Technical Query:", question)

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := g.client.GenerateContent(ctx, content)
	if err != nil {
		return question, err // Fallback to original question on error
	}

	if len(resp.Choices) == 0 {
		return question, nil
	}

	rewritten := strings.TrimSpace(resp.Choices[0].Content)

	// If the LLM returned something too short or failed, fallback
	if len(rewritten) < 3 {
		return question, nil
	}

	return rewritten, nil
}
