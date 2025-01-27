package replacer

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kechako/yomiko/ssml"
)

type dicEntry struct {
	from string
	to   string
}

type Replacer struct {
	dict []*dicEntry
}

func New(oldnew ...string) *Replacer {
	r := &Replacer{}
	r.build(oldnew)
	return r
}

func (r *Replacer) build(oldnew []string) {
	r.dict = make([]*dicEntry, 0, len(oldnew)/2)
	for i := 0; i+1 < len(oldnew); i += 2 {
		r.dict = append(r.dict, &dicEntry{
			from: oldnew[i],
			to:   oldnew[i+1],
		})
	}
}

var urlRegexp = regexp.MustCompile(`https?://[^\s]{2,}`)

func (r *Replacer) Replace(parent ssml.ParentNode, text string) {
	start := 0
	indexes := urlRegexp.FindAllStringIndex(text, -1)
	for _, index := range indexes {
		if start < index[0] {
			r.replaceText(parent, text[start:index[0]])
		}

		s := text[index[0]:index[1]]
		if strings.HasPrefix(s, "https://") {
			parent.AddNode(&ssml.SayAs{
				Text:        "https://",
				InterpretAs: "characters",
			})
			parent.AddNode(ssml.Text("以下略"))
		} else if strings.HasPrefix(s, "http://") {
			parent.AddNode(&ssml.SayAs{
				Text:        "http://",
				InterpretAs: "characters",
			})
			parent.AddNode(ssml.Text("以下略"))
		}

		start = index[1]
	}
	if start < len(text) {
		r.replaceText(parent, text[start:])
	}
}

type replaceNodes []ssml.Node

func (nodes *replaceNodes) AddNode(node ssml.Node) {
	*nodes = append(*nodes, node)
}

func (nodes *replaceNodes) AddNodes(n ...ssml.Node) {
	*nodes = append(*nodes, n...)
}

func (r *Replacer) replaceText(parent ssml.ParentNode, text string) {
	nodes := make(replaceNodes, 0, 16)

	replaceKusa(&nodes, text)

	for _, node := range nodes {
		if t, ok := node.(ssml.Text); ok {
			r.replaceDict(parent, 0, string(t))
		} else {
			parent.AddNode(node)
		}
	}
}

func (r *Replacer) replaceDict(parent ssml.ParentNode, entryIndex int, text string) {
	if entryIndex >= len(r.dict) {
		parent.AddNode(ssml.Text(text))
		return
	}
	entry := r.dict[entryIndex]

	for {
		before, after, found := strings.Cut(text, entry.from)
		if found {
			if before != "" {
				r.replaceDict(parent, entryIndex+1, before)
			}
			parent.AddNode(&ssml.Sub{
				Text:  ssml.Text(entry.from),
				Alias: entry.to,
			})
			text = after
		} else {
			break
		}
	}

	if text != "" {
		r.replaceDict(parent, entryIndex+1, text)
	}
}

var wwwRegexp = regexp.MustCompile(`([^wｗ]|^)([wｗ]+)`)

func replaceKusa(parent ssml.ParentNode, s string) {
	if len(s) == 0 {
		return
	}

	var textBuf bytes.Buffer
	textBuf.Reset()
	offset := 0
	for _, submatches := range wwwRegexp.FindAllStringSubmatchIndex(s, -1) {
		preceding := []rune(s[submatches[2]:submatches[3]])

		st, ed := submatches[4], submatches[5]
		kusa := s[st:ed]
		if offset < st {
			textBuf.WriteString(s[offset:st])
		}
		offset = ed

		var following []rune
		if ed < len(s) {
			following = []rune(s[ed:])[:1]
		}

		if (len(preceding) == 0 || !isAlphabet(preceding[0])) &&
			(len(following) == 0 || !isAlphabet(following[0])) {
			if textBuf.Len() > 0 {
				parent.AddNode(ssml.Text(textBuf.String()))
				textBuf.Reset()
			}
			parent.AddNode(&ssml.Sub{
				Text:  ssml.Text(kusa),
				Alias: makeKusa(kusa),
			})
		} else {
			textBuf.WriteString(kusa)
		}
	}
	if offset < len(s) {
		textBuf.WriteString(s[offset:])
	}
	if textBuf.Len() > 0 {
		parent.AddNode(ssml.Text(textBuf.String()))
		textBuf.Reset()
	}
}

func isAlphabet(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'ａ' && r <= 'ｚ') ||
		(r >= 'Ａ' && r <= 'Ｚ')
}

func makeKusa(kusa string) string {
	switch utf8.RuneCountInString(kusa) {
	case 0:
		return ""
	case 1:
		return "くさ"
	case 2:
		return "わらわら"
	case 3:
		return "わらわらわら"
	default:
		return "だいそうげん"
	}
}
