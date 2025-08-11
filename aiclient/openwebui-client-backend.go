package aiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otrace "go.opentelemetry.io/otel/trace"

	//"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	//"google.golang.org/grpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Global Variables
var apiURL = fmt.Sprintf("http://%s/ollama/v1/chat/completions",
	os.Getenv("OPEN_WEBUI_HOSTNAME"))

var apiKey = os.Getenv("OPEN_WEBUI_API_KEY")

var openLitEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_HOSTNAME")

var ctx context.Context
var webclient *http.Client
var tracer otrace.Tracer
var sessionSpan otrace.Span
var turnSpan otrace.Span
var chatHistory []AIMessage

// initTraceProvider creates a new trace provider and registers it globally.
func initTraceProvider() (*trace.TracerProvider, error) {

	log.Println("aiclient.initTraceProvider: called")

	// Create a new OTLP trace exporter using gRPC.
	ctx = context.Background()
	/*
		exporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(openLitEndpoint),
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		)
	*/
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(openLitEndpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create a new tracer provider with the OTLP exporter.
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("go-webchat"),
			attribute.String("environment", "development"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

// InitAIChat
func InitAIChat() {

	log.Println("aiclient.InitAIChat: called")

	// Set up OpenTelemetry trace provider.
	tp, err := initTraceProvider()
	if err != nil {
		log.Fatalf("aiclient.InitAIChat: failed to initialize trace provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("aiclient.InitAIChat: error shutting down tracer provider: %v", err)
		}
	}()

	// Get a tracer for our application.
	tracer = otel.Tracer("chat-app-tracer")

	// Start a root span for the entire chat session.
	ctx, sessionSpan = tracer.Start(context.Background(), "chat-session")
	defer sessionSpan.End()

	// Add a system message to set the context for the model.
	chatHistory = append(chatHistory, AIMessage{
		Role:    "system",
		Content: "You are a helpful assistant.",
	})

	// Create a custom HTTP client that skips TLS certificate verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// create http client with otelhttp
	webclient = &http.Client{
		Transport: otelhttp.NewTransport(tr),
	}

}

func PromptChat(userInput string) string {

	log.Printf("aiclient.PromptChat: called with prompt: %s", userInput)

	// Append user prompt to the chat history
	chatHistory = append(chatHistory, AIMessage{
		Role:    "user",
		Content: userInput,
	})

	// Create a new span for this specific chat turn
	tracer = otel.Tracer("chat-app-tracer")
	ctx, turnSpan = tracer.Start(ctx, "chat-turn")
	turnSpan.SetAttributes(attribute.String("user.input", userInput))

	// Create the request payload with mmodel name
	requestPayload := ChatRequest{
		Model:    "gemma:7B",
		Messages: chatHistory,
	}

	// Marshal the request payload into a JSON byte array
	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		turnSpan.RecordError(err)
		log.Fatalf("aiclient.PromptChat: Error marshaling JSON payload: %v", err)
	}

	// Create a new HTTP POST request (NewRequestWithContext for tracing)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		turnSpan.RecordError(err)
		log.Fatalf("aiclient.PromptChat: Error creating HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Send request..
	resp, err := webclient.Do(req)
	if err != nil {
		turnSpan.RecordError(err)
		log.Fatalf("aiclient.PromptChat: Error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors (turnSpan attribute for tracing)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		turnSpan.SetAttributes(attribute.String("http.response.status", resp.Status))
		log.Fatalf("aiclient.PromptChat: API request failed with status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		turnSpan.RecordError(err)
		log.Fatalf("aiclient.PromptChat: Error reading response body: %v", err)
	}

	// Unmarshal the JSON response into ChatResponse struct
	var responsePayload ChatResponse
	err = json.Unmarshal(body, &responsePayload)
	if err != nil {
		turnSpan.RecordError(err)
		log.Fatalf("aiclient.PromptChat: Error unmarshaling JSON response: %v", err)
	}

	// Check a response message was found
	if len(responsePayload.Choices) > 0 {
		assistantMessage := responsePayload.Choices[0].AIMessage
		turnSpan.SetAttributes(attribute.String("assistant.response", assistantMessage.Content))

		// Print the model's response
		log.Printf("aiclient.PromptChat: Response: %s\n", assistantMessage.Content)

		// Add the assistant's message to the chat history for context.
		chatHistory = append(chatHistory, assistantMessage)

		turnSpan.End()
		return assistantMessage.Content

	} else {
		log.Println("aiclient.PromptChat: Response: No response received.")
		turnSpan.End()
		return "No response received."
	}

}
