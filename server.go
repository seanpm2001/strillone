package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aetrion/dnsimple-go/dnsimple/webhook"
	"github.com/bluele/slack"
)

const what = "dnsimple-slackhooks"
const dnsimpleURL = "https://dnsimple.com"

var (
	httpPort        string
	slackWebhookURL string
	slackDryRun     bool
)

func init() {
	// for now read the URL from the ENV.
	// in the future we may probably want to be able to provide a flexible configuration.
	slackWebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	if slackWebhookURL == "" {
		log.Fatalln("Slack Webhook URL is missing")
	}

	slackDryRun = true
	if slackWebhookURL != "-" {
		slackDryRun = false
	}

	httpPort = os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "5000"
	}
}

func main() {
	log.Printf("Starting %s...\n", what)

	server := NewServer()

	log.Printf("%s listening on %s...\n", what, httpPort)
	if err := http.ListenAndServe(":"+httpPort, server); err != nil {
		log.Panic(err)
	}
}

// Server represents a front-end web server.
type Server struct {
	// Router which handles incoming requests
	mux *http.ServeMux
}

// NewServer returns a new front-end web server that handles HTTP requests for the app.
func NewServer() *Server {
	router := http.NewServeMux()
	server := &Server{mux: router}
	router.HandleFunc("/", server.Root)
	router.HandleFunc("/w", server.Webhook)
	return server
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Root is the handler for the HTTP requests to /.
// It returns a simple uptime message useful for monitoring.
func (s *Server) Root(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s\n", r.Method, r.URL.RequestURI())
	w.Header().Set("Content-type", "application/json")

	fmt.Fprintln(w, fmt.Sprintf(`{"ping":"%v","what":"%s"}`, time.Now().Unix(), what))
}

func (s *Server) Webhook(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s\n", r.Method, r.URL.RequestURI())

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var err error

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Error parsing body: %v\n", err)
	}

	event, err := webhook.Parse(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Error parsing event: %v\n", err)
	}

	text := EventText(event)
	log.Println(text)

	if !slackDryRun {
		log.Printf("Sending to slack...\n")

		webhook := slack.NewWebHook(slackWebhookURL)
		slackErr := webhook.PostMessage(&slack.WebHookPostPayload{Text: text})
		if slackErr != nil {
			log.Printf("Error sending to slack: %v\n", err)
		}
	}
}

func EventText(e webhook.Event) (text string) {
	//base  := e.(*webhook.GenericEvent)
	actor := fmt.Sprintf("%v from %v", "Someone", "<https://dnsimple.com|Awesome Company>")

	switch event := e.(type) {
	case *webhook.DomainCreateEvent:
		text = fmt.Sprintf("%s created the domain <https://dnsimple.com|%s>", actor, event.Domain.Name)
	default:
		text = fmt.Sprintf("%s performed an unknown action %s", actor, event.Event())
	}

	return
}