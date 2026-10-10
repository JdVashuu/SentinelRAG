package parser

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"go.opentelemetry.io/otel"
)

// CodeChunk represents a semantically meaningful block of code
type CodeChunk struct {
	Content   string
	FilePath  string
	StartLine int
	EndLine   int
	Type      string // e.g., "function", "class", "method"
}

// Parser handles the conversion of source files into searchable chunks
type Parser struct {
	goLang *sitter.Language
}

func NewParser() *Parser {
	return &Parser{
		goLang: golang.GetLanguage(),
	}
}

// ParseFile reads a file and attempts to break it into semantic chunks
func (p *Parser) ParseFile(ctx context.Context, filePath string) ([]CodeChunk, error) {
	_, span := otel.Tracer("parser").Start(ctx, "ParseFile")
	defer span.End()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if strings.HasSuffix(filePath, ".go") {
		return p.parseGoFile(ctx, filePath, content)
	}

	return p.simpleChunk(filePath, string(content)), nil
}

func (p *Parser) parseGoFile(ctx context.Context, filePath string, content []byte) ([]CodeChunk, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(p.goLang)

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}

	n := tree.RootNode()
	chunks := []CodeChunk{}

	// Iterate over top-level declarations
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		// We focus on functions and type declarations for now
		if child.Type() == "function_declaration" || child.Type() == "method_declaration" || child.Type() == "type_declaration" {
			chunks = append(chunks, CodeChunk{
				Content:   child.Content(content),
				FilePath:  filePath,
				StartLine: int(child.StartPoint().Row) + 1,
				EndLine:   int(child.EndPoint().Row) + 1,
				Type:      child.Type(),
			})
		}
	}

	return chunks, nil
}

func (p *Parser) simpleChunk(filePath, content string) []CodeChunk {
	// Placeholder: In a real implementation, this is where Tree-sitter
	// would identify function boundaries.
	return []CodeChunk{
		{
			Content:   content,
			FilePath:  filePath,
			StartLine: 1,
			EndLine:   -1, // Represents end of file
			Type:      "file",
		},
	}
}
