package kb

import (
	"strings"
	"unicode"

	"github.com/ajramos/giznews/internal/db"
)

// conceptBrands map a concept's canonical key to the spelling its owner uses.
// The classifier lowercases tags, so a concept derived from one arrives as
// "openai" or "hugging-face"; only a table can tell where the capitals go.
var conceptBrands = map[string]string{
	"openai":         "OpenAI",
	"chatgpt":        "ChatGPT",
	"deepmind":       "DeepMind",
	"googledeepmind": "Google DeepMind",
	"deepseek":       "DeepSeek",
	"huggingface":    "Hugging Face",
	"github":         "GitHub",
	"gitlab":         "GitLab",
	"pytorch":        "PyTorch",
	"tensorflow":     "TensorFlow",
	"langchain":      "LangChain",
	"llamaindex":     "LlamaIndex",
	"nvidia":         "NVIDIA",
	"xai":            "xAI",
	"metaai":         "Meta AI",
	"mistralai":      "Mistral AI",
	"stabilityai":    "Stability AI",
	"scaleai":        "Scale AI",
	"openrouter":     "OpenRouter",
	"vllm":           "vLLM",
	"sglang":         "SGLang",
	"arxiv":          "arXiv",
	"youtube":        "YouTube",
	"linkedin":       "LinkedIn",
	"tiktok":         "TikTok",
	"iphone":         "iPhone",
	"ios":            "iOS",
	"macos":          "macOS",
}

// conceptAcronyms are the tokens that are never title-cased: they are written
// as the field writes them.
var conceptAcronyms = map[string]string{
	"agi":   "AGI",
	"ai":    "AI",
	"api":   "API",
	"apis":  "APIs",
	"asi":   "ASI",
	"cli":   "CLI",
	"cnn":   "CNN",
	"cot":   "CoT",
	"cpu":   "CPU",
	"cuda":  "CUDA",
	"eu":    "EU",
	"gdpr":  "GDPR",
	"gpt":   "GPT",
	"gpu":   "GPU",
	"gpus":  "GPUs",
	"hbm":   "HBM",
	"jax":   "JAX",
	"kv":    "KV",
	"llm":   "LLM",
	"llms":  "LLMs",
	"lora":  "LoRA",
	"mcp":   "MCP",
	"ml":    "ML",
	"mlops": "MLOps",
	"moe":   "MoE",
	"nlp":   "NLP",
	"ocr":   "OCR",
	"onnx":  "ONNX",
	"qlora": "QLoRA",
	"rag":   "RAG",
	"rl":    "RL",
	"rlhf":  "RLHF",
	"rnn":   "RNN",
	"sdk":   "SDK",
	"slm":   "SLM",
	"ssm":   "SSM",
	"stt":   "STT",
	"tpu":   "TPU",
	"tts":   "TTS",
	"ui":    "UI",
	"uk":    "UK",
	"us":    "US",
	"ux":    "UX",
	"vlm":   "VLM",
	"vram":  "VRAM",
}

// DisplayName is the name a reader should see for a concept. Values that
// already carry capitals came from the model's entity extraction and are used
// verbatim; the lowercase ones come from tags or from a slug, and are expanded
// here — "rag" reads as RAG, "open-ai" as OpenAI, "gpt-5" keeps its hyphen
// because the number belongs to the word before it.
func DisplayName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ToLower(s) != s {
		return s
	}
	if brand, ok := conceptBrands[db.CanonKey(Slugify(s))]; ok {
		return brand
	}

	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(tokens) == 0 {
		return s
	}
	var b strings.Builder
	for i, tok := range tokens {
		if i > 0 {
			if isNumeric(tok) {
				b.WriteString("-")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(caseToken(tok))
	}
	return b.String()
}

func caseToken(tok string) string {
	if cased, ok := conceptAcronyms[tok]; ok {
		return cased
	}
	r := []rune(tok)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

func isNumeric(tok string) bool {
	for _, r := range tok {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return tok != ""
}
