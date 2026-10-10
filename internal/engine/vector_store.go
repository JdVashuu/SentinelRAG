package engine

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/vashuu/sentinel-rag/internal/parser"
	"go.opentelemetry.io/otel"
)

const CollectionName = "code_chunks"

type VectorStore struct {
	client *qdrant.Client
}

func NewVectorStore(host string, port int) (*VectorStore, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	return &VectorStore{client: client}, nil
}

func (vs *VectorStore) Init(ctx context.Context) error {
	collections, err := vs.client.ListCollections(ctx)
	if err != nil {
		return err
	}
	
	exists := false
	for _, c := range collections {
		if c == CollectionName {
			exists = true
			break
		}
	}

	if exists {
		return nil
	}

	// nomic-embed-text uses 768 dimensions (note: older versions used 1536, but nomic is 768)
	err = vs.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: CollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     768,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	return err
}

func (vs *VectorStore) AddChunks(ctx context.Context, chunks []VectorizedChunk) error {
	tracer := otel.Tracer("engine")
	ctx, span := tracer.Start(ctx, "AddChunks")
	defer span.End()

	points := make([]*qdrant.PointStruct, len(chunks))
	for i, chunk := range chunks {
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(uuid.New().String()),
			Vectors: qdrant.NewVectors(chunk.Embedding...),
			Payload: qdrant.NewValueMap(map[string]interface{}{
				"content":    chunk.Content,
				"file_path":  chunk.FilePath,
				"start_line": chunk.StartLine,
				"end_line":   chunk.EndLine,
				"type":       chunk.Type,
			}),
		}
	}

	_, err := vs.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CollectionName,
		Points:         points,
	})
	return err
}

func (vs *VectorStore) Search(ctx context.Context, queryVector []float32, limit uint64) ([]VectorizedChunk, error) {
	tracer := otel.Tracer("engine")
	ctx, span := tracer.Start(ctx, "SearchVectors")
	defer span.End()

	res, err := vs.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: CollectionName,
		Query:          qdrant.NewQuery(queryVector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}

	results := make([]VectorizedChunk, len(res))
	for i, point := range res {
		payload := point.Payload
		results[i] = VectorizedChunk{
			Embedding: nil, // We usually don't need the embedding back for display
			CodeChunk: parser.CodeChunk{
				Content:   payload["content"].GetStringValue(),
				FilePath:  payload["file_path"].GetStringValue(),
				StartLine: int(payload["start_line"].GetIntegerValue()),
				EndLine:   int(payload["end_line"].GetIntegerValue()),
				Type:      payload["type"].GetStringValue(),
			},
		}
	}

	return results, nil
}
