package ai

// Citation is one source behind a piece of generated text.
//
// Citations are attached to the [Text] they support rather than collected in
// one list on the response, because which sentence a source backs is the whole
// point of having sources at all. [Response.Citations] flattens them when the
// order is all that matters.
//
// An answer produced by a hosted search without citations cannot be checked,
// so a driver that receives sources from its provider always carries them
// through.
type Citation struct {
	// URL is where the source lives, and Title is what the provider called
	// it. Title is empty when the provider does not report one.
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`

	// CitedText is the fragment of the source that supports the text, as the
	// provider reported it. It is the source's words, not ours: it is what
	// makes a citation checkable without fetching the URL.
	CitedText string `json:"cited_text,omitempty"`

	// StartByte and EndByte bound the span of the enclosing Text.Text that
	// this source supports. Both zero means the source supports the whole
	// part, which is also what a provider that reports no offsets gets.
	//
	// They are byte offsets into a Go string, so a driver whose provider
	// counts in something else - runes, UTF-16 code units, tokens - converts
	// them or leaves them zero. Passing another provider's units through
	// would put the boundary mid-character on any text that is not ASCII,
	// and Ukrainian or Greek prose is not ASCII.
	StartByte int `json:"start_byte,omitempty"`
	EndByte   int `json:"end_byte,omitempty"`
}

// Citations returns every citation across the response's text parts, in the
// order the parts appear. It is a convenience for callers that only want the
// list of sources; the association between a source and the sentence it backs
// lives on [Text.Citations].
func (r *Response) Citations() []Citation {
	var out []Citation
	for _, p := range r.Parts {
		if t, ok := p.(Text); ok {
			out = append(out, t.Citations...)
		}
	}
	return out
}
