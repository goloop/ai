package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRequestValidate(t *testing.T) {
	if err := (&Request{}).Validate(); !errors.Is(err, ErrNoModel) {
		t.Errorf("empty model: got %v, want ErrNoModel", err)
	}
	if err := (&Request{Model: "m"}).Validate(); !errors.Is(err, ErrNoMessages) {
		t.Errorf("no messages: got %v, want ErrNoMessages", err)
	}
	ok := &Request{Model: "m", Messages: []Message{UserText("hi")}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid request: %v", err)
	}
}

func TestResponseText(t *testing.T) {
	r := &Response{Parts: []Part{
		Text{Text: "Hello "},
		ToolUse{Name: "t"},
		Text{Text: "world"},
	}}
	if got := r.Text(); got != "Hello world" {
		t.Errorf("Text() = %q", got)
	}
	if got := (&Response{}).Text(); got != "" {
		t.Errorf("empty Text() = %q", got)
	}
}

func TestResponseToolCalls(t *testing.T) {
	r := &Response{Parts: []Part{
		Text{Text: "x"},
		ToolUse{ID: "1", Name: "a", Input: json.RawMessage(`{}`)},
		ToolUse{ID: "2", Name: "b"},
	}}
	calls := r.ToolCalls()
	if len(calls) != 2 || calls[0].Name != "a" || calls[1].Name != "b" {
		t.Errorf("ToolCalls() = %+v", calls)
	}
}

func TestMessageHelpers(t *testing.T) {
	u := UserText("hi")
	if u.Role != RoleUser || len(u.Parts) != 1 {
		t.Errorf("UserText = %+v", u)
	}
	a := AssistantText("yo")
	if a.Role != RoleAssistant {
		t.Errorf("AssistantText = %+v", a)
	}
	if txt, ok := u.Parts[0].(Text); !ok || txt.Text != "hi" {
		t.Errorf("part = %+v", u.Parts[0])
	}
}

func TestNewOptionsDefaults(t *testing.T) {
	o := NewOptions("key")
	if o.APIKey != "key" {
		t.Errorf("APIKey = %q", o.APIKey)
	}
	if o.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", o.Timeout)
	}
	if o.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", o.MaxRetries)
	}
	if o.HTTPClient == nil {
		t.Error("HTTPClient not initialized")
	}
}

func TestNewOptionsOverrides(t *testing.T) {
	o := NewOptions("k",
		WithBaseURL("https://example"),
		WithTimeout(5*time.Second),
		WithMaxRetries(0),
		WithHeader("x-a", "1"),
		WithHeader("x-a", "2"),
	)
	if o.BaseURL != "https://example" {
		t.Errorf("BaseURL = %q", o.BaseURL)
	}
	if o.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d", o.MaxRetries)
	}
	if got := o.Headers.Values("x-a"); len(got) != 2 {
		t.Errorf("header x-a = %v, want two values", got)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	withMsg := &APIError{Status: 429, Message: "slow down"}
	if withMsg.Error() != "ai: api error 429: slow down" {
		t.Errorf("Error() = %q", withMsg.Error())
	}
	noMsg := &APIError{Status: 500}
	if noMsg.Error() != "ai: api error 500" {
		t.Errorf("Error() = %q", noMsg.Error())
	}
}

func TestRequestValidateNil(t *testing.T) {
	var r *Request
	if err := r.Validate(); !errors.Is(err, ErrNoRequest) {
		t.Fatalf("nil request: got %v, want ErrNoRequest", err)
	}
}

func TestDoGuardsNegativeRetriesAndNilClient(t *testing.T) {
	// A directly built Options with a negative retry count and no client must
	// not panic or silently return (nil, nil). Point it at an unroutable
	// address so the single attempt fails cleanly at the transport layer.
	o := Options{MaxRetries: -3}
	resp, err := o.Do(context.Background(), "GET", "http://127.0.0.1:0/", nil, nil)
	if resp != nil {
		resp.Body.Close()
		t.Fatal("expected no response for a failed transport attempt")
	}
	if err == nil {
		t.Fatal("expected a transport error, got nil (loop was skipped)")
	}
}

func TestOptionsStringRedactsAPIKey(t *testing.T) {
	o := NewOptions("sk-secret-value")
	if s := o.String(); strings.Contains(s, "sk-secret-value") {
		t.Fatalf("String leaked the API key: %s", s)
	}
	if s := fmt.Sprintf("%+v", o); strings.Contains(s, "sk-secret-value") {
		t.Fatalf("%%+v leaked the API key: %s", s)
	}
}

func TestMessageJSONRoundTrip(t *testing.T) {
	msg := Message{Role: RoleUser, Parts: []Part{
		Text{Text: "hello"},
		Image{MIME: "image/png", Data: []byte{1, 2, 3}},
		ToolUse{ID: "call_1", Name: "lookup", Input: json.RawMessage(`{"q":"x"}`)},
		ToolResult{ID: "call_1", Content: "done", IsError: true},
	}}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Role != RoleUser || len(got.Parts) != 4 {
		t.Fatalf("round-trip lost structure: %+v", got)
	}
	if _, ok := got.Parts[0].(Text); !ok {
		t.Fatalf("part 0 = %T, want Text", got.Parts[0])
	}
	tu, ok := got.Parts[2].(ToolUse)
	if !ok || string(tu.Input) != `{"q":"x"}` {
		t.Fatalf("tool_use round-trip: %+v", got.Parts[2])
	}
	tr, ok := got.Parts[3].(ToolResult)
	if !ok || tr.ID != "call_1" || !tr.IsError {
		t.Fatalf("tool_result round-trip: %+v", got.Parts[3])
	}
}

func TestMessageJSONUnknownPartRejected(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"role":"user","parts":[{"type":"mystery"}]}`), &m)
	if err == nil {
		t.Fatal("unknown part type should be rejected, not silently dropped")
	}
}

func TestRequestJSONRoundTrip(t *testing.T) {
	req := &Request{Model: "m", Messages: []Message{UserText("hi")}, MaxTokens: 10}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Model != "m" || len(got.Messages) != 1 || got.Messages[0].Role != RoleUser {
		t.Fatalf("round-trip lost request: %+v", got)
	}
}

func TestResponseJSONRoundTrip(t *testing.T) {
	resp := Response{
		Model:      "m",
		Parts:      []Part{Text{Text: "hi"}, ToolUse{ID: "c1", Name: "f", Input: json.RawMessage(`{}`)}},
		StopReason: "stop",
		Usage:      Usage{InputTokens: 3, OutputTokens: 2},
		Raw:        json.RawMessage(`{"provider":"x"}`),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"input_tokens":3`) {
		t.Fatalf("usage not snake_case: %s", data)
	}
	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Model != "m" || got.StopReason != "stop" || got.Usage.OutputTokens != 2 {
		t.Fatalf("round-trip lost fields: %+v", got)
	}
	if got.Text() != "hi" || len(got.ToolCalls()) != 1 {
		t.Fatalf("round-trip lost parts: %+v", got)
	}
}

func TestOptionsGoStringRedacts(t *testing.T) {
	o := NewOptions("sk-xyz")
	if s := fmt.Sprintf("%#v", o); strings.Contains(s, "sk-xyz") {
		t.Fatalf("%%#v leaked the API key: %s", s)
	}
}
