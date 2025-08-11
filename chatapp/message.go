package chatapp

type Message struct {
	ClientID string `json:"clientID"`
	Prompt   string `json:"text"`
	Response string `json:"response"`
}
