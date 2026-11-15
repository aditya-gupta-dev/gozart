package seo

import "github.com/ollama/ollama/api"

type Agent interface {
	GetTools() []api.Tool
}
