package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/goloop/ai"
)

// mockClient is a trivial in-memory [ai.Client]. A real driver sends the
// request to a provider; this shows the contract Generate and Stream must
// satisfy - the same shape every goloop AI provider implements.
type mockClient struct{}

func (mockClient) Generate(ctx context.Context, req *ai.Request) (*ai.Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &ai.Response{
		Model:      req.Model,
		Parts:      []ai.Part{ai.Text{Text: "Hello!"}},
		StopReason: "stop",
		Usage:      ai.Usage{InputTokens: 3, OutputTokens: 2},
	}, nil
}

func (mockClient) Stream(ctx context.Context, req *ai.Request) iter.Seq2[ai.Chunk, error] {
	return func(yield func(ai.Chunk, error) bool) {
		if err := req.Validate(); err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		for _, part := range []string{"Hel", "lo!"} {
			if !yield(ai.Chunk{Text: part}, nil) {
				return
			}
		}
		yield(ai.Chunk{Done: true, Usage: &ai.Usage{InputTokens: 3, OutputTokens: 2}}, nil)
	}
}

func ExampleClient_generate() {
	var c ai.Client = mockClient{}

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model: "demo",
		Messages: []ai.Message{
			ai.SystemText("You are concise."),
			ai.UserText("Say hi."),
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Text())
	fmt.Printf("%d in / %d out\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	// Output:
	// Hello!
	// 3 in / 2 out
}

func ExampleClient_stream() {
	var c ai.Client = mockClient{}

	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model:    "demo",
		Messages: []ai.Message{ai.UserText("Say hi.")},
	}) {
		if err != nil {
			panic(err)
		}
		fmt.Print(chunk.Text)
	}
	fmt.Println()
	// Output: Hello!
}

func ExampleResponse_ToolCall() {
	resp := &ai.Response{Parts: []ai.Part{
		ai.Text{Text: "let me check"},
		ai.ToolUse{ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Kyiv"}`)},
	}}

	if call, ok := resp.ToolCall("get_weather"); ok {
		fmt.Printf("%s(%s)\n", call.Name, call.Input)
	}
	// Output: get_weather({"city":"Kyiv"})
}

func ExampleTool() {
	tool := ai.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}
	fmt.Println(tool.Name)
	// Output: get_weather
}
