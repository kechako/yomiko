package bot

import (
	"context"
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/kechako/yomiko/tts"
)

func (b *Bot) getApplicationCommands(ctx context.Context) ([]discord.ApplicationCommandCreate, error) {
	voices, err := b.tts.ListVoices(ctx)
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.getApplicationCommands: %w", err)
	}
	voiceChoices := make(map[string][]discord.ApplicationCommandOptionChoiceString)
	for _, voice := range voices {
		var gender string
		switch voice.SsmlGender {
		case tts.GenderMale:
			gender = "male"
		case tts.GenderFemale:
			gender = "female"
		case tts.GenderNeutral:
			// 現在 neutral は ja-JP に存在しないみたいなので使用しない
			continue
		}

		choice := discord.ApplicationCommandOptionChoiceString{
			Name:  voice.Name,
			Value: voice.Name,
		}

		voiceChoices[gender] = append(voiceChoices[gender], choice)
	}

	var (
		minSpeed = float64(tts.MinSpeakingRate)
		maxSpeed = float64(tts.MaxSpeakingRate)
		minPitch = float64(tts.MinPitch)
		maxPitch = float64(tts.MaxPitch)
	)

	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "yomiko",
			Description: "読子さんに指示を出します。",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "join",
					Description: "読子さんをボイスチャンネルに入室させます。",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionChannel{
							Name:         "voice-channel",
							Description:  "読子さんが入室するボイスチャンネル。",
							ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildVoice},
							Required:     true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "leave",
					Description: "読子さんをボイスチャンネルから退室させます。",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "male-voice",
					Description: "読子さんの声を男性に変更します。",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "読子さんの声。",
							Choices:     voiceChoices["male"],
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "female-voice",
					Description: "読子さんの声を女性に変更します。",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "name",
							Description: "読子さんの声。",
							Choices:     voiceChoices["female"],
							Required:    true,
						},
					},
				},
				// 現在 neutral は ja-JP に存在しないみたいなので使用しない
				//discord.ApplicationCommandOptionSubCommand{
				//	Name:        "neutral-voice",
				//	Description: "読子さんの声を中性に変更します。",
				//	Options: []discord.ApplicationCommandOption{
				//		discord.ApplicationCommandOptionString{
				//			Name:        "name",
				//			Description: "読子さんの声。",
				//			Choices:     voiceChoices["neutral"],
				//			Required:    true,
				//		},
				//	},
				//},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "speed",
					Description: "読子さんの読み上げ速度を変更します。",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionFloat{
							Name:        "speed",
							Description: "読子さんの読み上げ速度。",
							MinValue:    &minSpeed,
							MaxValue:    &maxSpeed,
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "pitch",
					Description: "読子さんの声の音程を変更します。",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionFloat{
							Name:        "pitch",
							Description: "読子さんの声の音程。",
							MinValue:    &minPitch,
							MaxValue:    &maxPitch,
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "reset",
					Description: "読子さんの声の設定を初期値に設定します。",
				},
			},
		},
	}, nil
}
