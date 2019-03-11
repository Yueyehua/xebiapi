package main

import (
  "fmt"
  "html/template"
  "log"
  "net/http"
  "time"
)

/*
 * This structure keeps track of the clients connections.
 * clients is a map of channels to send messages with.
 * newClients is a channel for new clients.
 * remClients is a channel for clients to be disconnected.
 * messages is a channel for messages to be sent.
 */
type Broker struct {
  clients map[chan string]bool
  newClients chan chan string
  remClients chan chan string
  messages chan string
}

/*
 * This starts the broker to be run as go routine.
 * It handles openning and closing clients so as messages.
 */
func (b *Broker) Run() {
	for {
		select {
		case s := <-b.newClients:
			b.clients[s] = true
			log.Println("New client connection")
		case s := <-b.remClients:
			delete(b.clients, s)
			close(s)
			log.Println("Removed client")
		case msg := <-b.messages:
			for s := range b.clients { s <- msg }
			log.Printf("Broadcast message to %d clients", len(b.clients))
		}
  }
}

/*
 * This handles the http requests of the /events/ endpoint.
 * It is required by http.Handle method.
 */
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
  }
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
  w.Header().Set("Transfer-Encoding", "chunked")
  msgChan := make(chan string)
  b.newClients <- msgChan
  notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		b.remClients <- msgChan
		log.Println("HTTP connection just closed.")
  }()
	for {
    msg, open := <-msgChan
    if !open { break }
    fmt.Fprintf(w, "data: Message: %s\n\n", msg)
    f.Flush()
  }
  log.Println("Closed connection at ", r.URL.Path)
}

/*
 * This is a handler for http requests of the / endpoint.
 */
func handler(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	t, err := template.ParseFiles("templates/index.html")
	if err != nil { log.Fatal("Template parse error.") }
	t.Execute(w, "friend")
	log.Println("Closing connection at", r.URL.Path)
}

/*
 * This is the main go routine.
 */
func main() {
	b := &Broker{
		make(map[chan string]bool),
		make(chan (chan string)),
		make(chan (chan string)),
		make(chan string),
	}
  go b.Run()
  http.Handle("/events/", b)
	go func() {
		for { // TODO: Add a function to receive events from external source.
      time.Sleep(time.Second * 2)
			log.Println("Receiving event")
      b.messages <- fmt.Sprintf("the time is %v", time.Now())
		}
  }()
  http.Handle("/", http.HandlerFunc(handler))
  if err := http.ListenAndServe(":80", nil); err != nil { panic(err) }
}
