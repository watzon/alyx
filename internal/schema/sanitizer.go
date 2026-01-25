package schema

import (
	"github.com/microcosm-cc/bluemonday"
)

func NewRichTextSanitizer(config *RichTextConfig) *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowStandardURLs()

	allowedFormats := config.GetAllowedFormats()
	formatSet := make(map[RichTextFormat]bool)
	for _, f := range allowedFormats {
		formatSet[f] = true
	}

	p.AllowElements("p", "br")

	if formatSet[RichTextFormatBold] {
		p.AllowElements("strong", "b")
	}

	if formatSet[RichTextFormatItalic] {
		p.AllowElements("em", "i")
	}

	if formatSet[RichTextFormatUnderline] {
		p.AllowElements("u")
	}

	if formatSet[RichTextFormatStrike] {
		p.AllowElements("s", "strike", "del")
	}

	if formatSet[RichTextFormatCode] {
		p.AllowElements("code")
	}

	if formatSet[RichTextFormatLink] {
		p.AllowAttrs("href", "target", "rel").OnElements("a")
		p.AllowRelativeURLs(true)
		p.RequireNoFollowOnLinks(false)
	}

	if formatSet[RichTextFormatHeading] {
		p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	}

	if formatSet[RichTextFormatBlockquote] {
		p.AllowElements("blockquote")
	}

	if formatSet[RichTextFormatCodeBlock] {
		p.AllowElements("pre")
		p.AllowAttrs("class").OnElements("pre", "code")
	}

	if formatSet[RichTextFormatBulletList] {
		p.AllowElements("ul", "li")
	}

	if formatSet[RichTextFormatOrderedList] {
		p.AllowElements("ol", "li")
	}

	if formatSet[RichTextFormatHorizontalRule] {
		p.AllowElements("hr")
	}

	return p
}

func SanitizeRichText(html string, config *RichTextConfig) string {
	if config == nil {
		config = &RichTextConfig{Preset: RichTextPresetBasic}
	}
	p := NewRichTextSanitizer(config)
	return p.Sanitize(html)
}
