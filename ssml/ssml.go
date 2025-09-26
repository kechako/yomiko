package ssml

import (
	"bufio"
	"encoding/xml"
	"io"
	"strings"
)

type Node interface {
	encode(enc *xml.Encoder) error
	write(w *bufio.Writer) error
}

type ParentNode interface {
	AddNode(node Node)
	AddNodes(node ...Node)
}

type SSML struct {
	Nodes []Node
}

func New() *SSML {
	return &SSML{}
}

var (
	_ Node       = (*SSML)(nil)
	_ ParentNode = (*SSML)(nil)
)

func (ssml *SSML) AddNode(node Node) {
	ssml.Nodes = append(ssml.Nodes, node)
}

func (ssml *SSML) AddNodes(nodes ...Node) {
	ssml.Nodes = append(ssml.Nodes, nodes...)
}

func (ssml *SSML) WriteSSML(w io.Writer) error {
	enc := xml.NewEncoder(w)

	err := ssml.encode(enc)
	if err != nil {
		return err
	}

	err = enc.Close()
	if err != nil {
		return err
	}

	return nil
}

func (ssml *SSML) WriteText(w io.Writer) error {
	var bw *bufio.Writer
	if ww, ok := w.(*bufio.Writer); ok {
		bw = ww
	} else {
		bw = bufio.NewWriter(w)
	}

	err := ssml.write(bw)
	if err != nil {
		return err
	}

	err = bw.Flush()
	if err != nil {
		return err
	}

	return nil
}

func (ssml *SSML) ToSSML() string {
	var s strings.Builder
	err := ssml.WriteSSML(&s)
	if err != nil {
		panic("bug: " + err.Error())
	}
	return s.String()
}

func (ssml *SSML) ToText() string {
	var s strings.Builder
	err := ssml.WriteText(&s)
	if err != nil {
		panic("bug: " + err.Error())
	}
	return s.String()
}

var (
	speakName     = xml.Name{Local: "speak"}
	paragraphName = xml.Name{Local: "p"}
	sentenceName  = xml.Name{Local: "s"}
	sayAsName     = xml.Name{Local: "say-as"}
	subName       = xml.Name{Local: "sub"}
)

func (ssml *SSML) encode(enc *xml.Encoder) error {
	var err error

	err = enc.EncodeToken(xml.StartElement{
		Name: speakName,
	})
	if err != nil {
		return err
	}

	for _, node := range ssml.Nodes {
		err := node.encode(enc)
		if err != nil {
			return err
		}
	}

	err = enc.EncodeToken(xml.EndElement{
		Name: speakName,
	})
	if err != nil {
		return err
	}

	return nil
}

func (ssml *SSML) write(w *bufio.Writer) error {
	for _, node := range ssml.Nodes {
		err := node.write(w)
		if err != nil {
			return err
		}
	}

	return nil
}

type Paragraph struct {
	Nodes []Node
}

var (
	_ Node       = (*Paragraph)(nil)
	_ ParentNode = (*Paragraph)(nil)
)

func (p *Paragraph) AddNode(node Node) {
	p.Nodes = append(p.Nodes, node)
}

func (p *Paragraph) AddNodes(nodes ...Node) {
	p.Nodes = append(p.Nodes, nodes...)
}

func (p *Paragraph) encode(enc *xml.Encoder) error {
	var err error

	err = enc.EncodeToken(xml.StartElement{
		Name: paragraphName,
	})
	if err != nil {
		return err
	}

	for _, node := range p.Nodes {
		err := node.encode(enc)
		if err != nil {
			return err
		}
	}

	err = enc.EncodeToken(xml.EndElement{
		Name: paragraphName,
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Paragraph) write(w *bufio.Writer) error {
	for _, node := range p.Nodes {
		err := node.write(w)
		if err != nil {
			return err
		}
	}

	return nil
}

type Sentence struct {
	Nodes []Node
}

var (
	_ Node       = (*Sentence)(nil)
	_ ParentNode = (*Sentence)(nil)
)

func (s *Sentence) AddNode(node Node) {
	_, ok := node.(*Paragraph)
	if ok {
		panic("Sentence cloud not contain paragraph")
	}
	s.Nodes = append(s.Nodes, node)
}

func (s *Sentence) AddNodes(nodes ...Node) {
	for _, node := range nodes {
		s.AddNode(node)
	}
}

func (s *Sentence) encode(enc *xml.Encoder) error {
	var err error

	err = enc.EncodeToken(xml.StartElement{
		Name: sentenceName,
	})
	if err != nil {
		return err
	}

	for _, node := range s.Nodes {
		err := node.encode(enc)
		if err != nil {
			return err
		}
	}

	err = enc.EncodeToken(xml.EndElement{
		Name: sentenceName,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Sentence) write(w *bufio.Writer) error {
	for _, node := range s.Nodes {
		err := node.write(w)
		if err != nil {
			return err
		}
	}

	return nil
}

type Text string

var _ Node = Text("")

func (t Text) encode(enc *xml.Encoder) error {
	return enc.EncodeToken(xml.CharData(t))
}

func (t Text) write(w *bufio.Writer) error {
	_, err := w.WriteString(string(t))
	if err != nil {
		return err
	}
	return nil
}

type InterpretationType string

const (
	Currency   InterpretationType = "currency"
	Telephone  InterpretationType = "telephone"
	Verbatim   InterpretationType = "verbatim"
	SpellOut   InterpretationType = "spell-out"
	Date       InterpretationType = "date"
	Characters InterpretationType = "characters"
	Cardinal   InterpretationType = "cardinal"
	Ordinal    InterpretationType = "ordinal"
	Expletive  InterpretationType = "expletive"
	Bleep      InterpretationType = "bleep"
	Unit       InterpretationType = "unit"
)

type SayAs struct {
	Text        Text
	InterpretAs InterpretationType
	Format      string
	Detail      string
	Language    string
}

var _ Node = (*SayAs)(nil)

func (sa *SayAs) encode(enc *xml.Encoder) error {
	var err error

	err = enc.EncodeToken(xml.StartElement{
		Name: sayAsName,
		Attr: sa.attrs(),
	})
	if err != nil {
		return err
	}

	err = sa.Text.encode(enc)
	if err != nil {
		return err
	}

	err = enc.EncodeToken(xml.EndElement{
		Name: sayAsName,
	})
	if err != nil {
		return err
	}

	return nil
}

func (sa *SayAs) write(w *bufio.Writer) error {
	return sa.Text.write(w)
}

func (sa *SayAs) attrs() []xml.Attr {
	attrs := make([]xml.Attr, 0, 3)
	attrs = append(attrs, xml.Attr{
		Name:  xml.Name{Local: "interpret-as"},
		Value: string(sa.InterpretAs),
	})

	switch sa.InterpretAs {
	case Currency:
		if sa.Language != "" {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "language"},
				Value: sa.Language,
			})
		}
	case Date:
		if sa.Format != "" {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "format"},
				Value: sa.Format,
			})
		}
		if sa.Detail != "" {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "detail"},
				Value: sa.Detail,
			})
		}
	}

	return attrs
}

type Sub struct {
	Text  Text
	Alias string
}

var _ Node = (*Sub)(nil)

func (sub *Sub) encode(enc *xml.Encoder) error {
	var err error

	err = enc.EncodeToken(xml.StartElement{
		Name: subName,
		Attr: []xml.Attr{
			{
				Name:  xml.Name{Local: "alias"},
				Value: sub.Alias,
			},
		},
	})
	if err != nil {
		return err
	}

	err = sub.Text.encode(enc)
	if err != nil {
		return err
	}

	err = enc.EncodeToken(xml.EndElement{
		Name: subName,
	})
	if err != nil {
		return err
	}

	return nil
}

func (sub *Sub) write(w *bufio.Writer) error {
	_, err := w.WriteString(sub.Alias)
	if err != nil {
		return err
	}
	return nil
}
