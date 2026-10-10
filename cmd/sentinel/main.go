package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/vashuu/sentinel-rag/internal/config"
	"github.com/vashuu/sentinel-rag/internal/engine"
	"github.com/vashuu/sentinel-rag/internal/parser"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func initTracer(cfg *config.Config) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Connect to Jaeger OTLP gRPC collector
	conn, err := grpc.DialContext(ctx, cfg.JaegerEndpoint, 
		grpc.WithTransportCredentials(insecure.NewCredentials()), 
		grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func main() {
	// Load .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	cfg := config.Load()

	// Setup Observability
	tp, err := initTracer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Initialize components
	p := parser.NewParser()
	ingestor := engine.NewIngestor(p)
	
	// Using Ollama with nomic-embed-text for local embeddings
	embedder, err := engine.NewOllamaEmbedder(cfg.EmbedderModel)
	if err != nil {
		log.Printf("Warning: Failed to initialize Ollama embedder: %v. AI features will be disabled.", err)
	}

	// Setup Qdrant
	vstore, err := engine.NewVectorStore(cfg.QdrantHost, cfg.QdrantPort) // gRPC port
	if err != nil {
		log.Fatalf("Failed to initialize vector store: %v", err)
	}
	if err := vstore.Init(context.Background()); err != nil {
		log.Printf("Warning: Could not initialize Qdrant collection (is it running?): %v", err)
	}

	// Initialize Generator for RAG (using phi3 - lightweight and fast)
	generator, err := engine.NewGenerator(cfg.GeneratorModel)
	if err != nil {
		log.Printf("Warning: Failed to initialize Generator: %v. Answers will not be generated.", err)
	}

	r := gin.Default()

	// Health Check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Trigger Ingestion & Embedding
	r.POST("/ingest", func(c *gin.Context) {
		var req struct {
			Directory string `json:"directory" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		tracer := otel.Tracer("sentinel-rag")
		ctx, span := tracer.Start(ctx, "IngestAndEmbed")
		defer span.End()

		// 1. Ingest
		chunks, err := ingestor.IngestDirectory(ctx, req.Directory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ingestion failed: " + err.Error()})
			return
		}

		// 2. Embed
		if embedder == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Embedder not initialized"})
			return
		}
		vectorized, err := embedder.EmbedChunks(ctx, chunks)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Embedding failed: " + err.Error()})
			return
		}

		// 3. Store in Qdrant
		if err := vstore.AddChunks(ctx, vectorized); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store vectors: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"directory":    req.Directory,
			"chunks_found": len(chunks),
			"stored":       len(vectorized),
		})
	})

	// Semantic Search Query + AI Generation
	r.POST("/query", func(c *gin.Context) {
		var req struct {
			Question string `json:"question" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		tracer := otel.Tracer("sentinel-rag")
		ctx, span := tracer.Start(ctx, "QueryAndAnswer")
		defer span.End()

		// 1. Self-Healing: Rewrite the query for better retrieval
		searchQuery := req.Question
		if generator != nil {
			rewritten, err := generator.RewriteQuery(ctx, req.Question)
			if err == nil {
				searchQuery = rewritten
				log.Printf("Rewritten query: %s -> %s", req.Question, searchQuery)
			}
		}

		// 2. Embed the optimized search query
		queryVec, err := embedder.EmbedChunks(ctx, []parser.CodeChunk{{Content: searchQuery}})
		if err != nil || len(queryVec) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to embed question"})
			return
		}

		// 3. Search Qdrant for context
		matches, err := vstore.Search(ctx, queryVec[0].Embedding, 5)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed: " + err.Error()})
			return
		}

		// 3. Generate answer using retrieved context
		var answer string
		if generator != nil && len(matches) > 0 {
			answer, err = generator.GenerateAnswer(ctx, req.Question, matches)
			if err != nil {
				log.Printf("Generation error: %v", err)
				answer = "Error generating answer, but context was found."
			}
		} else {
			answer = "No context found to generate an answer."
		}

		c.JSON(http.StatusOK, gin.H{
			"answer":  answer,
			"context": matches,
		})
	})

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")
}
