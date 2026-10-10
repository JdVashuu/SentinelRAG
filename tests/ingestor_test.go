package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vashuu/sentinel-rag/internal/engine"
	"github.com/vashuu/sentinel-rag/internal/parser"
)

func TestIngestDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	goContent := "package test\nfunc Hello() {}"
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(goContent), 0644)

	p := parser.NewParser()
	ingestor := engine.NewIngestor(p)

	chunks, err := ingestor.IngestDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("IngestDirectory failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks, got 0")
	}
}
