package main

import (
	"flag"
	"log"
	"net/http"
	"ollama-copilot/internal"
	"ollama-copilot/internal/handlers"
	"ollama-copilot/internal/middleware"
	"text/template"

	"github.com/ollama/ollama/api"
)

var (
	port        = flag.String("port", ":9090", "Port to listen on")
	proxyPort   = flag.String("proxy-port", ":8080", "Proxy port to listen on")
	cert        = flag.String("cert", "server.crt", "Certificate file path *.crt")
	key         = flag.String("key", "server.key", "Key file path *.key")
	model       = flag.String("model", "codellama:code", "LLM model to use")
	templateStr = flag.String("template", "<PRE> {{.Prefix}} <SUF>{{.Suffix}} <MID>", "Fill-in-middle template to apply in prompt")
)

// main is the entrypoint for the program.
func main() {
	flag.Parse()
	api, err := api.ClientFromEnvironment()

	if err != nil {
		log.Fatalf("error initialize api: %s", err.Error())
		return
	}

	templ, err := template.New("prompt").Parse(*templateStr)
	if err != nil {
		log.Fatalf("error parsing template: %s", err.Error())
		return
	}

	mux := http.NewServeMux()

	mux.Handle("/health", handlers.NewHealthHandler())
	mux.Handle("/copilot_internal/v2/token", handlers.NewTokenHandler())
	mux.Handle("/v1/engines/copilot-codex/completions", handlers.NewCompletionHandler(api, *model, templ))

	go internal.Proxy(*proxyPort)

	http.ListenAndServeTLS(*port, *cert, *key, middleware.LogMiddleware(mux))
}