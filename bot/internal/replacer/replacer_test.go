package replacer

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kechako/yomiko/ssml"
)

var replaceKusaTests = []struct {
	in    string
	nodes []ssml.Node
}{
	{
		in: "あいうえおw",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "w", Alias: "くさ"},
		},
	},
	{
		in: "あいうえおｗ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ｗ", Alias: "くさ"},
		},
	},
	{
		in: "あいうえおww",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ww", Alias: "わらわら"},
		},
	},
	{
		in: "あいうえおｗｗ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ｗｗ", Alias: "わらわら"},
		},
	},
	{
		in: "あいうえおwww",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "www", Alias: "わらわらわら"},
		},
	},
	{
		in: "あいうえおｗｗｗ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ｗｗｗ", Alias: "わらわらわら"},
		},
	},
	{
		in: "あいうえおwwww",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "wwww", Alias: "だいそうげん"},
		},
	},
	{
		in: "あいうえおｗｗｗｗ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ｗｗｗｗ", Alias: "だいそうげん"},
		},
	},
	{
		in: "あいうえおwwwかきくけこ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "www", Alias: "わらわらわら"},
			ssml.Text("かきくけこ"),
		},
	},
	{
		in: "あいうえおｗｗｗかきくけこ",
		nodes: []ssml.Node{
			ssml.Text("あいうえお"),
			&ssml.Sub{Text: "ｗｗｗ", Alias: "わらわらわら"},
			ssml.Text("かきくけこ"),
		},
	},
	{
		in: "wwwかきくけこ",
		nodes: []ssml.Node{
			&ssml.Sub{Text: "www", Alias: "わらわらわら"},
			ssml.Text("かきくけこ"),
		},
	},
	{
		in: "ｗｗｗかきくけこ",
		nodes: []ssml.Node{
			&ssml.Sub{Text: "ｗｗｗ", Alias: "わらわらわら"},
			ssml.Text("かきくけこ"),
		},
	},
	{
		in: "abcdefwww",
		nodes: []ssml.Node{
			ssml.Text("abcdefwww"),
		},
	},
	{
		in: "ａｂｃｄｅｆｗｗｗ",
		nodes: []ssml.Node{
			ssml.Text("ａｂｃｄｅｆｗｗｗ"),
		},
	},
	{
		in: "wwwabcdef",
		nodes: []ssml.Node{
			ssml.Text("wwwabcdef"),
		},
	},
	{
		in: "ｗｗｗａｂｃｄｅｆ",
		nodes: []ssml.Node{
			ssml.Text("ｗｗｗａｂｃｄｅｆ"),
		},
	},
	{
		in: "abcdefwwwghijk",
		nodes: []ssml.Node{
			ssml.Text("abcdefwwwghijk"),
		},
	},
	{
		in: "ａｂｃｄｅｆｗｗｗｇｈｉｊｋ",
		nodes: []ssml.Node{
			ssml.Text("ａｂｃｄｅｆｗｗｗｇｈｉｊｋ"),
		},
	},
	{
		in: "abcdef www ghijk",
		nodes: []ssml.Node{
			ssml.Text("abcdef "),
			&ssml.Sub{Text: "www", Alias: "わらわらわら"},
			ssml.Text(" ghijk"),
		},
	},
	{
		in: "ａｂｃｄｅｆ　ｗｗｗ　ｇｈｉｊｋ",
		nodes: []ssml.Node{
			ssml.Text("ａｂｃｄｅｆ　"),
			&ssml.Sub{Text: "ｗｗｗ", Alias: "わらわらわら"},
			ssml.Text("　ｇｈｉｊｋ"),
		},
	},
	{
		in: "w ww www wwww",
		nodes: []ssml.Node{
			&ssml.Sub{Text: "w", Alias: "くさ"},
			ssml.Text(" "),
			&ssml.Sub{Text: "ww", Alias: "わらわら"},
			ssml.Text(" "),
			&ssml.Sub{Text: "www", Alias: "わらわらわら"},
			ssml.Text(" "),
			&ssml.Sub{Text: "wwww", Alias: "だいそうげん"},
		},
	},
}

func TestReplaceKusa(t *testing.T) {
	for i, tt := range replaceKusaTests {
		t.Run(fmt.Sprintf("test_%02d", i+1), func(t *testing.T) {
			var nodes replaceNodes

			replaceKusa(&nodes, tt.in)
			if diff := cmp.Diff(tt.nodes, []ssml.Node(nodes)); diff != "" {
				t.Errorf("replaceKusa(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}

var replacerTests = []struct {
	in    string
	nodes []ssml.Node
}{
	{
		in: "ああああ超電磁砲いいいいhttp://www.example.com/ ううううｗｗｗｗええええhttps://www.example.com/ 禁書目録おおおお",
		nodes: []ssml.Node{
			ssml.Text("ああああ"),
			&ssml.Sub{Text: "超電磁砲", Alias: "れーるがん"},
			ssml.Text("いいいい"),
			&ssml.SayAs{Text: "http://", InterpretAs: ssml.Characters},
			ssml.Text("以下略"),
			ssml.Text(" うううう"),
			&ssml.Sub{Text: "ｗｗｗｗ", Alias: "だいそうげん"},
			ssml.Text("ええええ"),
			&ssml.SayAs{Text: "https://", InterpretAs: ssml.Characters},
			ssml.Text("以下略"),
			ssml.Text(" "),
			&ssml.Sub{Text: "禁書目録", Alias: "いんでっくす"},
			ssml.Text("おおおお"),
		},
	},
	{
		in: "ああああ超電磁砲禁書目録いいいいｗｗｗｗうううう禁書目録ｗｗｗｗ超電磁砲おおおお",
		nodes: []ssml.Node{
			ssml.Text("ああああ"),
			&ssml.Sub{Text: "超電磁砲", Alias: "れーるがん"},
			&ssml.Sub{Text: "禁書目録", Alias: "いんでっくす"},
			ssml.Text("いいいい"),
			&ssml.Sub{Text: "ｗｗｗｗ", Alias: "だいそうげん"},
			ssml.Text("うううう"),
			&ssml.Sub{Text: "禁書目録", Alias: "いんでっくす"},
			&ssml.Sub{Text: "ｗｗｗｗ", Alias: "だいそうげん"},
			&ssml.Sub{Text: "超電磁砲", Alias: "れーるがん"},
			ssml.Text("おおおお"),
		},
	},
}

func TestReplacerReplace(t *testing.T) {
	r := New("禁書目録", "いんでっくす", "超電磁砲", "れーるがん")
	for i, tt := range replacerTests {
		t.Run(fmt.Sprintf("test_%02d", i+1), func(t *testing.T) {
			var nodes replaceNodes

			r.Replace(&nodes, tt.in)
			if diff := cmp.Diff(tt.nodes, []ssml.Node(nodes)); diff != "" {
				t.Errorf("Replacer.Replace(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}
