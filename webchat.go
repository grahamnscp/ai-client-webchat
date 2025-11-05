package main

import (
	"log"
	"net/http"

  "webchat/chatapp"
)

func serveIndex(w http.ResponseWriter, r *http.Request) {
	//if r.URL.Path != "/" {
	//	http.Error(w, "not found", http.StatusNotFound)
	//	return
	//}

	if r.Method != "GET" {
		http.Error(w, "method not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, "templates/index.html")
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

func main() {
	broker := chatapp.NewMsgBroker()
	go broker.Run()

	http.HandleFunc("/", serveIndex)
  http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		chatapp.ServeWs(broker, w, r)
	})

  log.Print("Serve Http on 8086")
	log.Fatal(http.ListenAndServe(":8086", nil))
}
