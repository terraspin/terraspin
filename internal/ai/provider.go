package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Provider executes requests for one LLM service.
type Provider interface {
	Name() string
	NeedsKey() bool
	KeyEnv() string
	Query(context.Context, Request) (Response, error)
}

// Request is the provider-neutral input for an LLM request.
type Request struct {
	Model  string
	System string
	Prompt string
	APIKey string
	Extra  map[string]string
}

// Response is the provider-neutral result of an LLM request.
type Response struct {
	Text   string
	Tokens int
}

var registry = map[string]Provider{}

func registerProvider(provider Provider) {
	if _, exists := registry[provider.Name()]; exists {
		panic("duplicate LLM provider: " + provider.Name())
	}
	registry[provider.Name()] = provider
}

// Lookup returns the registered provider named name.
func Lookup(name string) (Provider, bool) {
	provider, ok := registry[name]
	return provider, ok
}

// Names returns registered provider names in alphabetical order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func unknownProviderError(name string) error {
	return fmt.Errorf("unknown LLM provider: %s (registered: %s)", name, strings.Join(Names(), ", "))
}

// QueryLLM routes to a registered provider and returns its text and tokens.
func QueryLLM(ctx context.Context, provider, apiKey, model, host, prompt string) (string, int, error) {
	registered, ok := Lookup(provider)
	if !ok {
		return "", 0, unknownProviderError(provider)
	}
	response, err := registered.Query(ctx, Request{
		Model: model, System: SystemPrompt, Prompt: prompt, APIKey: apiKey,
		Extra: map[string]string{"ollama_host": host},
	})
	return response.Text, response.Tokens, err
}
