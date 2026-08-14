package llm

import (
	"fmt"
	"time"
)

// Options configures the provider selected by NewProvider. It is intentionally
// a plain struct (not config.Config) so the llm package stays decoupled.
type Options struct {
	Provider string // ollama | openai | anthropic | bedrock | custom
	Model    string
	Endpoint string // base URL
	Region   string // bedrock only
	APIKey   string
	Timeout  time.Duration
}

// NewProvider builds the provider matching opts.Provider.
func NewProvider(opts Options) (Provider, error) {
	if opts.Endpoint == "" {
		opts.Endpoint = defaultEndpoint(opts.Provider)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 120 * time.Second
	}

	switch opts.Provider {
	case "", "ollama":
		return &ollamaProvider{endpoint: opts.Endpoint, timeout: opts.Timeout}, nil
	case "openai", "custom":
		return &openAIProvider{name: opts.Provider, endpoint: opts.Endpoint, apiKey: opts.APIKey, timeout: opts.Timeout}, nil
	case "anthropic":
		return &anthropicProvider{endpoint: opts.Endpoint, apiKey: opts.APIKey, timeout: opts.Timeout}, nil
	case "bedrock":
		return nil, fmt.Errorf("provider %q: %w", opts.Provider, ErrNotImplemented)
	default:
		return nil, fmt.Errorf("unknown LLM provider %q", opts.Provider)
	}
}

func defaultEndpoint(provider string) string {
	switch provider {
	case "ollama", "":
		return "http://localhost:11434"
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com"
	default:
		return ""
	}
}
