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
	voiceChoices := make([]*discordgo.ApplicationCommandOptionChoice, len(voices))
	for i, voice := range voices {
		var gender string
		switch voice.SsmlGender {
		case tts.GenderMale:
			gender = "男性"
		case tts.GenderFemale:
			gender = "女性"
		case tts.GenderNeutral:
			gender = "中性"
		}

		voiceChoices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s (%s)", voice.Name, gender),
			Value: voice.Name,
		}
	}

	var (
		minSpeed = float64(0.25)
		maxSpeed = float64(4.0)
		minPitch = float64(-20.0)
		maxPitch = float64(20.0)
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
					Name:        "voice",
					Description: "読子さんの声を変更します。",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "voice",
							Description: "読子さんの声。",
							Type:        discordgo.ApplicationCommandOptionString,
							Choices:     voiceChoices,
							Required:    true,
						},
					},
				},
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
			},
		},
	}, nil
}
