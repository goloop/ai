package ai

import (
	"encoding/json"
	"errors"
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
