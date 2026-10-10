package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vashuu/sentinel-rag/internal/parser"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Ingestor handles the orchestration of reading a codebase and converting it to chunks
type Ingestor struct {
	parser *parser.Parser
}

func NewIngestor(p *parser.Parser) *Ingestor {
	return &Ingestor{
		parser: p,
	}
}

// IngestResult holds the chunks from a file or an error
type IngestResult struct {
	FilePath string
	Chunks   []parser.CodeChunk
	Error    error
}

// IngestDirectory crawls a directory and returns all semantic chunks found
func (i *Ingestor) IngestDirectory(ctx context.Context, rootDir string) ([]parser.CodeChunk, error) {
	tracer := otel.Tracer("engine")
	ctx, span := tracer.Start(ctx, "IngestDirectory")
	defer span.End()
	span.SetAttributes(attribute.String("directory", rootDir))

	var allChunks []parser.CodeChunk
	var wg sync.WaitGroup
	resultChan := make(chan IngestResult)

	// Walk the directory
	go func() {
		err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories and hidden files
			if info.IsDir() || info.Name()[0] == '.' {
				return nil
			}

			// For now, focus on Go files
			if filepath.Ext(path) == ".go" {
				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					chunks, parseErr := i.parser.ParseFile(ctx, p)
					resultChan <- IngestResult{
						FilePath: p,
						Chunks:   chunks,
						Error:    parseErr,
					}
				}(path)
			}
			return nil
		})

		if err != nil {
			resultChan <- IngestResult{Error: err}
		}

		// Close channel once all files are processed
		go func() {
			wg.Wait()
			close(resultChan)
		}()
	}()

	// Collect results
	for res := range resultChan {
		if res.Error != nil {
			// We log but continue for single file errors
			fmt.Printf("Warning: failed to ingest %s: %v\n", res.FilePath, res.Error)
			continue
		}
		allChunks = append(allChunks, res.Chunks...)
	}

	span.SetAttributes(attribute.Int("total_chunks", len(allChunks)))
	return allChunks, nil
}
