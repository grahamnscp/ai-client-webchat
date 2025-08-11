package aiclient

// Message represents a single message in the chat, with a role and content
// The role can be "user", "assistant", or "system"
type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the JSON payload sent to the OpenWebUI API
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []AIMessage `json:"messages"`
}

// ChatResponse represents the JSON response received from the API
type ChatResponse struct {
	Choices []struct {
		AIMessage AIMessage `json:"message"`
	} `json:"choices"`
}

