package tests

import (
	"context"
	"testing"

	"github.com/vashuu/sentinel-rag/internal/engine"
)

func TestQueryRewriterLogic(t *testing.T) {
	g, err := engine.NewGenerator("phi3")
	if err != nil {
		t.Skip("Ollama/phi3 not available")
	}

	ctx := context.Background()
	question := "how it starts?"
	
	rewritten, err := g.RewriteQuery(ctx, question)
	if err != nil {
		t.Logf("Rewriter failed (expected if offline): %v", err)
		return
	}

	if rewritten == "" {
		t.Error("Rewriter returned an empty string")
	}
	
	t.Logf("Original: %s", question)
	t.Logf("Rewritten: %s", rewritten)
}
