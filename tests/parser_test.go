package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vashuu/sentinel-rag/internal/parser"
)

func TestParseGoFile(t *testing.T) {
	p := parser.NewParser()
	ctx := context.Background()

	// Create a dummy file in the tests directory for parsing
	content := `package test
	func TestMe() string { return "passed" }`
	
	tmpFile := filepath.Join(t.TempDir(), "dummy.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	chunks, err := p.ParseFile(ctx, tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected chunks, got 0")
	}
}
