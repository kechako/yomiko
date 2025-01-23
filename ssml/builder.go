package ssml

import "strings"

type Builder struct {
	b strings.Builder
}

func New() *Builder {
	return &Builder{}
}

var ssmlEscaper = strings.NewReplacer(
	`"`, "&quot;",
	`&`, "&amp;",
	`'`, "&apos;",
	`<`, "&lt;",
	`>`, "&gt;",
)

func (b *Builder) Text(text string) {
	ssmlEscaper.WriteString(&b.b, text)
}

func (b *Builder) SayAs(interpretAs string, text string) {
	b.b.WriteString("<say-as interpret-as=\"")
	ssmlEscaper.WriteString(&b.b, interpretAs)
	b.b.WriteString("\">")
	ssmlEscaper.WriteString(&b.b, text)
	b.b.WriteString("</say-as>")
}

func (b *Builder) Sub(text, alias string) {
	b.b.WriteString("<sub alias=\"")
	ssmlEscaper.WriteString(&b.b, alias)
	b.b.WriteString("\">")
	ssmlEscaper.WriteString(&b.b, text)
	b.b.WriteString("</sub>")
}

func (b *Builder) Paragraph(f func(b *Builder)) {
	b.b.WriteString("<p>")
	f(b)
	b.b.WriteString("</p>")
}

func (b *Builder) Sentence(f func(b *Builder)) {
	b.b.WriteString("<s>")
	f(b)
	b.b.WriteString("</s>")
}

func (b *Builder) String() string {
	return "<speak>" + b.b.String() + "</speak>"
}
