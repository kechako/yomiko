package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/kechako/yomiko/tts"
)

func (bot *Bot) getApplicationCommands(ctx context.Context) ([]*discordgo.ApplicationCommand, error) {
	voices, err := bot.tts.ListVoices(ctx)
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.getApplicationCommands: %w", err)
	}
	voiceChoices := make(map[string][]*discordgo.ApplicationCommandOptionChoice)
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

		choice := &discordgo.ApplicationCommandOptionChoice{
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

	return []*discordgo.ApplicationCommand{
		{
			Name:        "yomiko",
			Description: "読子さんに指示を出します。",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "join",
					Description: "読子さんをボイスチャンネルに入室させます。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:         "voice-channel",
							Description:  "読子さんが入室するボイスチャンネル。",
							Type:         discordgo.ApplicationCommandOptionChannel,
							ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildVoice},
							Required:     true,
						},
					},
				},
				{
					Name:        "leave",
					Description: "読子さんをボイスチャンネルから退室させます。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "male-voice",
					Description: "読子さんの声を男性に変更します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "name",
							Description: "読子さんの声。",
							Type:        discordgo.ApplicationCommandOptionString,
							Choices:     voiceChoices["male"],
							Required:    true,
						},
					},
				},
				{
					Name:        "female-voice",
					Description: "読子さんの声を女性に変更します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "name",
							Description: "読子さんの声。",
							Type:        discordgo.ApplicationCommandOptionString,
							Choices:     voiceChoices["female"],
							Required:    true,
						},
					},
				},
				// 現在 neutral は ja-JP に存在しないみたいなので使用しない
				//{
				//	Name:        "neutral-voice",
				//	Description: "読子さんの声を中性に変更します。",
				//	Type:        discordgo.ApplicationCommandOptionSubCommand,
				//	Options: []*discordgo.ApplicationCommandOption{
				//		{
				//			Name:        "name",
				//			Description: "読子さんの声。",
				//			Type:        discordgo.ApplicationCommandOptionString,
				//			Choices:     voiceChoices["neutral"],
				//			Required:    true,
				//		},
				//	},
				//},
				{
					Name:        "speed",
					Description: "読子さんの読み上げ速度を変更します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "speed",
							Description: "読子さんの読み上げ速度。",
							Type:        discordgo.ApplicationCommandOptionNumber,
							MinValue:    &minSpeed,
							MaxValue:    maxSpeed,
							Required:    true,
						},
					},
				},
				{
					Name:        "pitch",
					Description: "読子さんの声の音程を変更します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "pitch",
							Description: "読子さんの声の音程。",
							Type:        discordgo.ApplicationCommandOptionNumber,
							MinValue:    &minPitch,
							MaxValue:    maxPitch,
							Required:    true,
						},
					},
				},
				{
					Name:        "reset",
					Description: "読子さんの声の設定を初期値に設定します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
	}, nil
}
