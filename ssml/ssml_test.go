package ssml

import "testing"

func TestSSML(t *testing.T) {
	root := New()

	p := &Paragraph{}
	root.AddNode(p)

	s := &Sentence{}
	p.AddNode(s)

	s.AddNodes(
		Text("aaaa"),
		Text("bbbb"),
		&SayAs{
			Text:        Text("ABCDE"),
			InterpretAs: Characters,
		},
		Text("cccc"),
		&Sub{
			Text:  Text("禁書目録"),
			Alias: "いんでっくす",
		},
		Text("dddd"),
	)

	const want = `<speak><p><s>aaaabbbb<say-as interpret-as="characters">ABCDE</say-as>cccc<sub alias="いんでっくす">禁書目録</sub>dddd</s></p></speak>`

	got := root.ToSSML()
	if got != want {
		t.Errorf("SSML.ToSSML():\ngot : %s\nwant: %s", got, want)
	}
}
