package aiclient

import (
	"log"
)

func AIClientQuery(message string) string {

	log.Printf("aiclient.AIClientQuery: called with prompt: '%s'", message)

	InitAIChat()
	aiResponse := PromptChat(message)

	log.Printf("aiclient.AIClientQuery: received response: '%s'", aiResponse)

	return aiResponse
}
