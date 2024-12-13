package neo

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"github.com/yaoapp/gou/process"
	"github.com/yaoapp/yao/neo/assistant"
)

// HookCreate create the assistant
func (neo *DSL) HookCreate(ctx Context, messages []map[string]interface{}, c *gin.Context) error {
	if neo.Create == "" {
		return nil
	}

	// Create a context with 10 second timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	p, err := process.Of(neo.Create, ctx, messages, c.Writer)
	if err != nil {
		return err
	}

	err = p.WithContext(timeoutCtx).Execute()
	if err != nil {
		return err
	}
	defer p.Release()

	// Check if context was canceled
	if timeoutCtx.Err() != nil {
		return timeoutCtx.Err()
	}

	return nil
}

// HookAssistants query the assistant list from the assistant list hook
func (neo *DSL) HookAssistants(ctx context.Context, param assistant.QueryParam) ([]assistant.Assistant, error) {
	if neo.AssistantListHook == "" {
		return nil, nil
	}

	// Create a context with 10 second timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	p, err := process.Of(neo.AssistantListHook, param)
	if err != nil {
		return nil, err
	}

	err = p.WithContext(timeoutCtx).Execute()
	if err != nil {
		return nil, err
	}
	defer p.Release()

	// Check if context was canceled
	if timeoutCtx.Err() != nil {
		return nil, timeoutCtx.Err()
	}

	value := p.Value()
	if value == nil {
		return nil, nil
	}

	var list []assistant.Assistant
	bytes, err := jsoniter.Marshal(value)
	if err != nil {
		return nil, err
	}

	err = jsoniter.Unmarshal(bytes, &list)
	if err != nil {
		return nil, err
	}

	return list, nil
}

// HookPrepare executes the prepare hook before AI is called
func (neo *DSL) HookPrepare(ctx Context, messages []map[string]interface{}) ([]map[string]interface{}, error) {
	if neo.Prepare == "" {
		return messages, nil
	}

	// Create a context with 10 second timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	p, err := process.Of(neo.Prepare, ctx, messages)
	if err != nil {
		return nil, err
	}

	err = p.WithContext(timeoutCtx).Execute()
	if err != nil {
		return nil, err
	}
	defer p.Release()

	// Check if context was canceled
	if timeoutCtx.Err() != nil {
		return nil, timeoutCtx.Err()
	}

	value := p.Value()
	if value == nil {
		return messages, nil
	}

	var result []map[string]interface{}
	bytes, err := jsoniter.Marshal(value)
	if err != nil {
		return nil, err
	}

	err = jsoniter.Unmarshal(bytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// HookWrite executes the write hook when response is received from AI
func (neo *DSL) HookWrite(ctx Context, messages []map[string]interface{}, response map[string]interface{}, content string, writer *gin.ResponseWriter) ([]map[string]interface{}, error) {
	if neo.Write == "" {
		return []map[string]interface{}{response}, nil
	}

	p, err := process.Of(neo.Write, ctx, messages, response, content, writer)
	if err != nil {
		return nil, err
	}

	err = p.WithContext(ctx).Execute()
	if err != nil {
		return nil, err
	}
	defer p.Release()

	value := p.Value()
	if value == nil {
		return []map[string]interface{}{response}, nil
	}

	var result []map[string]interface{}
	bytes, err := jsoniter.Marshal(value)
	if err != nil {
		return nil, err
	}

	err = jsoniter.Unmarshal(bytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
