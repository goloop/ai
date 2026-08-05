package ai

import (
	"errors"
	"strings"
	"testing"
)

// reply builds a Response whose text is s.
func reply(s string) *Response {
	return &Response{Parts: []Part{Text{Text: s}}}
}

// TestResponseJSON covers what a provider actually sends back when it was only
// asked for JSON in the prompt: the value, sometimes wrapped in a fence, and
// occasionally buried in prose - which is where decoding must stop rather than
// guess.
func TestResponseJSON(t *testing.T) {
	type doc struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	want := doc{A: "x", B: 7}

	good := []struct {
		name string
		text string
	}{
		{"bare value", `{"a":"x","b":7}`},
		{"surrounding whitespace", "\n  {\"a\":\"x\",\"b\":7}  \n"},
		{"json fence", "```json\n{\"a\":\"x\",\"b\":7}\n```"},
		{"bare fence", "```\n{\"a\":\"x\",\"b\":7}\n```"},
		{"fence with a language and spaces", "```JSON  \n{\"a\":\"x\",\"b\":7}\n```"},
		{"fence on one line", "```{\"a\":\"x\",\"b\":7}```"},
		{"fence whose info line is the value", "```{\"a\":\"x\",\"b\":7}\n```"},
		{"fence with trailing newline", "```json\n{\"a\":\"x\",\"b\":7}\n```\n"},
		{"pretty printed", "{\n  \"a\": \"x\",\n  \"b\": 7\n}"},
	}
	for _, c := range good {
		t.Run(c.name, func(t *testing.T) {
			var got doc
			if err := reply(c.text).JSON(&got); err != nil {
				t.Fatalf("JSON() = %v, want it to decode", err)
			}
			if got != want {
				t.Errorf("decoded %+v, want %+v", got, want)
			}
		})
	}

	bad := []struct {
		name string
		text string
	}{
		{"prose before", `Sure! Here it is: {"a":"x","b":7}`},
		{"prose after", `{"a":"x","b":7} Hope that helps!`},
		{"fence inside prose", "Here:\n```json\n{\"a\":\"x\"}\n```\nEnjoy!"},
		{"two values", `{"a":"x"} {"a":"y"}`},
		{"two fenced blocks", "```json\n{\"a\":\"x\"}\n```\n```json\n{\"a\":\"y\"}\n```"},
		{"not json at all", "I cannot do that."},
		{"truncated", `{"a":"x",`},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			var got doc
			if err := reply(c.text).JSON(&got); err == nil {
				t.Errorf("JSON() accepted %q as %+v", c.text, got)
			}
		})
	}
}

// TestResponseJSONNoText covers a reply with nothing to decode, such as one
// made up entirely of tool calls.
func TestResponseJSONNoText(t *testing.T) {
	var v any
	for _, r := range []*Response{
		{},
		reply("   \n  "),
		{Parts: []Part{ToolUse{ID: "1", Name: "t"}}},
	} {
		if err := r.JSON(&v); !errors.Is(err, ErrNoText) {
			t.Errorf("JSON() = %v, want %v", err, ErrNoText)
		}
	}
}

// TestResponseJSONArrayAndScalar checks the helper is not object-only: an
// array is the shape a "suggest ideas" prompt comes back in.
func TestResponseJSONArrayAndScalar(t *testing.T) {
	var list []string
	if err := reply("```json\n[\"a\",\"b\"]\n```").JSON(&list); err != nil {
		t.Fatalf("JSON() = %v", err)
	}
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Errorf("decoded %v, want [a b]", list)
	}

	for _, text := range []string{
		"42",
		"```json\n42\n```",
		"```\n42\n```",
		// No line follows, so the first line is the value, not an info string.
		"```42\n```",
		"```42```",
	} {
		var n int
		if err := reply(text).JSON(&n); err != nil || n != 42 {
			t.Errorf("JSON(%q) = %v, n = %d, want 42", text, err, n)
		}
	}
}

// TestUnfence pins the unwrapping on its own, including the cases where it
// must decline: half a fence is not a fence.
func TestUnfence(t *testing.T) {
	cases := map[string]string{
		"```json\n{}\n```": "{}",
		"```\n{}\n```":     "{}",
		"```{}```":         "{}",
		"{}":               "{}",
		"```":              "```",
		"``````":           "",
		"```json\n{}":      "```json\n{}",
		"```42\n```":       "42",
		"```json\n```":     "json",
		"{}\n```":          "{}\n```",
		// An info string that looks like content is content: do not drop it.
		"```\n[1,\n2]\n```": "[1,\n2]",
	}
	for in, want := range cases {
		if got := unfence(in); got != want {
			t.Errorf("unfence(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestResponseJSONIgnoresMode checks the decoder does not care how the format
// was satisfied: text that is JSON decodes whatever produced it.
func TestResponseJSONIgnoresMode(t *testing.T) {
	for _, mode := range []FormatMode{FormatNone, FormatNative, FormatEmulated} {
		r := reply(`{"a":"x"}`)
		r.Format = mode
		var v struct {
			A string `json:"a"`
		}
		if err := r.JSON(&v); err != nil || v.A != "x" {
			t.Errorf("mode %s: JSON() = %v, a = %q", mode, err, v.A)
		}
	}
}

// TestResponseJSONMultiPartText checks the decoder sees the whole reply, not
// just its first block, since drivers may split text across parts.
func TestResponseJSONMultiPartText(t *testing.T) {
	r := &Response{Parts: []Part{
		Text{Text: `{"a":`},
		Text{Text: `"x"}`},
	}}
	var v struct {
		A string `json:"a"`
	}
	if err := r.JSON(&v); err != nil {
		t.Fatalf("JSON() = %v", err)
	}
	if v.A != "x" {
		t.Errorf("a = %q, want %q", v.A, "x")
	}
}

// TestResponseJSONErrorMentionsPackage keeps the failure readable at the call
// site, where the caller has no idea which of nine drivers produced it.
func TestResponseJSONErrorMentionsPackage(t *testing.T) {
	var v any
	err := reply("nope").JSON(&v)
	if err == nil || !strings.HasPrefix(err.Error(), "ai: ") {
		t.Errorf("error = %v, want it prefixed with the package name", err)
	}
}
