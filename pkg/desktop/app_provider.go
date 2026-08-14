package desktop

import (
	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/llm"
)

// buildProvider constructs the LLM provider from config. Returns a nil provider
// (without error) when LLM is disabled, so callers can fall back to
// deterministic behavior.
func buildProvider(cfg *config.Config) (llm.Provider, error) {
	if !cfg.LLM.Enabled {
		return nil, nil
	}
	return llm.NewProvider(llm.Options{
		Provider: cfg.LLM.Provider,
		Model:    cfg.LLM.Model,
		Endpoint: cfg.LLM.Endpoint,
		Region:   cfg.LLM.Region,
		APIKey:   cfg.LLM.APIKey,
		Timeout:  cfg.LLMTimeout(),
	})
}

func (a *App) provider() (llm.Provider, error) {
	if a.prov != nil {
		return a.prov, nil
	}
	return buildProvider(a.cfg)
}
