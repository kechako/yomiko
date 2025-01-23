package ssml

import (
	"encoding/xml"
	"io"
	"strings"
)

type Node interface {
	encode(enc *xml.Encoder) error
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

func (ssml *SSML) WriteSSML(w io.Writer) {
	enc := xml.NewEncoder(w)

	err := ssml.encode(enc)
	if err != nil {
		panic("bug: " + err.Error())
	}

	err = enc.Close()
	if err != nil {
		panic("bug: " + err.Error())
	}
}

func (ssml *SSML) ToSSML() string {
	var s strings.Builder
	ssml.WriteSSML(&s)
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

type Text string

var _ Node = Text("")

func (t Text) encode(enc *xml.Encoder) error {
	return enc.EncodeToken(xml.CharData(t))
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
