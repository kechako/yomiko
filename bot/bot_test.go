package bot

import (
	"fmt"
	"testing"

	"github.com/kechako/yomiko/ssml"
)

var replaceKusaTests = []struct {
	in  string
	out string
}{
	{
		in:  "あいうえおw",
		out: "<speak>あいうえお<sub alias=\"くさ\">w</sub></speak>",
	},
	{
		in:  "あいうえおｗ",
		out: "<speak>あいうえお<sub alias=\"くさ\">ｗ</sub></speak>",
	},
	{
		in:  "あいうえおww",
		out: "<speak>あいうえお<sub alias=\"わらわら\">ww</sub></speak>",
	},
	{
		in:  "あいうえおｗｗ",
		out: "<speak>あいうえお<sub alias=\"わらわら\">ｗｗ</sub></speak>",
	},
	{
		in:  "あいうえおwww",
		out: "<speak>あいうえお<sub alias=\"わらわらわら\">www</sub></speak>",
	},
	{
		in:  "あいうえおｗｗｗ",
		out: "<speak>あいうえお<sub alias=\"わらわらわら\">ｗｗｗ</sub></speak>",
	},
	{
		in:  "あいうえおwwww",
		out: "<speak>あいうえお<sub alias=\"だいそうげん\">wwww</sub></speak>",
	},
	{
		in:  "あいうえおｗｗｗｗ",
		out: "<speak>あいうえお<sub alias=\"だいそうげん\">ｗｗｗｗ</sub></speak>",
	},
	{
		in:  "あいうえおwwwかきくけこ",
		out: "<speak>あいうえお<sub alias=\"わらわらわら\">www</sub>かきくけこ</speak>",
	},
	{
		in:  "あいうえおｗｗｗかきくけこ",
		out: "<speak>あいうえお<sub alias=\"わらわらわら\">ｗｗｗ</sub>かきくけこ</speak>",
	},
	{
		in:  "wwwかきくけこ",
		out: "<speak><sub alias=\"わらわらわら\">www</sub>かきくけこ</speak>",
	},
	{
		in:  "ｗｗｗかきくけこ",
		out: "<speak><sub alias=\"わらわらわら\">ｗｗｗ</sub>かきくけこ</speak>",
	},
	{
		in:  "abcdefwww",
		out: "<speak>abcdefwww</speak>",
	},
	{
		in:  "ａｂｃｄｅｆｗｗｗ",
		out: "<speak>ａｂｃｄｅｆｗｗｗ</speak>",
	},
	{
		in:  "wwwabcdef",
		out: "<speak>wwwabcdef</speak>",
	},
	{
		in:  "ｗｗｗａｂｃｄｅｆ",
		out: "<speak>ｗｗｗａｂｃｄｅｆ</speak>",
	},
	{
		in:  "abcdefwwwghijk",
		out: "<speak>abcdefwwwghijk</speak>",
	},
	{
		in:  "ａｂｃｄｅｆｗｗｗｇｈｉｊｋ",
		out: "<speak>ａｂｃｄｅｆｗｗｗｇｈｉｊｋ</speak>",
	},
	{
		in:  "abcdef www ghijk",
		out: "<speak>abcdef <sub alias=\"わらわらわら\">www</sub> ghijk</speak>",
	},
	{
		in:  "ａｂｃｄｅｆ　ｗｗｗ　ｇｈｉｊｋ",
		out: "<speak>ａｂｃｄｅｆ　<sub alias=\"わらわらわら\">ｗｗｗ</sub>　ｇｈｉｊｋ</speak>",
	},
}

func TestReplaceKusa(t *testing.T) {
	for i, tt := range replaceKusaTests {
		t.Run(fmt.Sprintf("test_%02d", i+1), func(t *testing.T) {
			b := ssml.New()
			replaceKusa(tt.in, b)
			got := b.String()
			if got != tt.out {
				t.Errorf("replaceKusa(%q): got %s, want %s", tt.in, got, tt.out)
			}
		})
	}
}
