package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/ollama/ollama/api"
)

// CompletionRequest is the request sent to the completion handler
type CompletionRequest struct {
	Extra struct {
		Language          string `json:"language"`
		NextIndent        int    `json:"next_indent"`
		PromptTokens      int    `json:"prompt_tokens"`
		SuffixTokens      int    `json:"suffix_tokens"`
		TrimByIndentation bool   `json:"trim_by_indentation"`
	} `json:"extra"`
	MaxTokens   int      `json:"max_tokens"`
	N           int      `json:"n"`
	Prompt      string   `json:"prompt"`
	Stop        []string `json:"stop"`
	Stream      bool     `json:"stream"`
	Suffix      string   `json:"suffix"`
	Temperature float64  `json:"temperature"`
	TopP        int      `json:"top_p"`
}

// Logprobs is the logprobs returned by the CompletionResponse
type Logprobs struct {
	Tokens []struct {
		Token   string  `json:"token"`
		Logprob float64 `json:"logprob"`
	} `json:"tokens"`
}

// ChoiceResponse is the response returned CompletionResponse
type ChoiceResponse struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	Logprobs     *Logprobs
	FinishReason string `json:"finish_reason"`
}

// CompletionResponse is the response returned by the CompletionHandler
type CompletionResponse struct {
	Id      string           `json:"id"`
	Created int64            `json:"created"`
	Choices []ChoiceResponse `json:"choices"`
}

// Prompt is an repreentation of a prompt with suffi and prefix
type Prompt struct {
	Prefix string
	Suffix string
}

func (p Prompt) Generate(templ *template.Template) string {
	var buf = new(bytes.Buffer)
	err := templ.Execute(buf, p)
	if err != nil {
		log.Printf("error executing prompt template: %s", err.Error())
	}

	return buf.String()
}

// CompletionHandler is an http.Handler that returns completions.
type CompletionHandler struct {
	api   *api.Client
	model string
	templ *template.Template
}

// NewCompletionHandler returns a new CompletionHandler.
func NewCompletionHandler(api *api.Client, model string, template *template.Template) *CompletionHandler {
	return &CompletionHandler{api, model, template}
}

// ServeHTTP implements http.Handler.
func (c *CompletionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req := CompletionRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Fatalf("error decode: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	generate := api.GenerateRequest{
		Model:  "codellama:code",
		Prompt: Prompt{Prefix: req.Prompt, Suffix: req.Suffix}.Generate(c.templ),
		Options: map[string]interface{}{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"stop":        req.Stop,
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	err := c.api.Generate(r.Context(), &generate, func(resp api.GenerateResponse) error {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := CompletionResponse{
			Id:      uuid.New().String(),
			Created: time.Now().Unix(),
			Choices: []ChoiceResponse{
				{
					Text:  resp.Response,
					Index: 0,
				},
			},
		}

		w.Write([]byte("data: "))
		json.NewEncoder(w).Encode(response)

		if resp.Done {
			w.Write([]byte("data: [DONE]"))
			wg.Done()
		}
		return nil
	})

	wg.Wait()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error generating completion: %v", err)
		return
	}
}