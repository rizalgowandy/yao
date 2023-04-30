package neo

import (
	"github.com/yaoapp/yao/aigc"
	"github.com/yaoapp/yao/neo/conversation"
)

// DSL AI assistant
type DSL struct {
	ID                  string                 `json:"-" yaml:"-"`
	Name                string                 `json:"name,omitempty"`
	Guard               string                 `json:"guard,omitempty"`
	Connector           string                 `json:"connector"`
	ConversationSetting conversation.Setting   `json:"conversation" yaml:"conversation"`
	Option              map[string]interface{} `json:"option"`
	Prompts             []aigc.Prompt          `json:"prompts,omitempty"`
	Allows              []string               `json:"allows,omitempty"`
	AI                  aigc.AI                `json:"-" yaml:"-"`
	Conversation        Conversation           `json:"-" yaml:"-"`
	Command             Command                `json:"-" yaml:"-"`
}

// Conversation the store interface
type Conversation interface {
	GetHistory(sid string) ([]map[string]interface{}, error)
	SaveHistory(sid string, messages []map[string]interface{}) error
}

// Command the command interface
type Command interface {
	Match(messages []map[string]interface{}) (bool, error)
}
