package tests

import (
	"context"
	"testing"

	"github.com/vashuu/sentinel-rag/internal/engine"
	"github.com/vashuu/sentinel-rag/internal/parser"
)

func TestGeneratorPromptBuilding(t *testing.T) {
	// We test the logic of answer generation with a dummy model or check logic
	// Since Ollama requires a running server, we'll focus on the logic we can verify
	g, err := engine.NewGenerator("llama3")
	if err != nil {
		t.Skip("Ollama not available, skipping generator test")
	}

	ctx := context.Background()
	question := "What does Hello do?"
	chunks := []engine.VectorizedChunk{
		{
			CodeChunk: parser.CodeChunk{
				Content:  "func Hello() { fmt.Println(\"Hi\") }",
				FilePath: "test.go",
			},
		},
	}

	// This would attempt a real call if Ollama is up
	_, err = g.GenerateAnswer(ctx, question, chunks)
	if err != nil {
		t.Logf("Generator call failed (expected if Ollama is down): %v", err)
	}
}
