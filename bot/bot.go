package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/kechako/yomiko/tts"
)

var (
	errYomikoAlreadyJoined = errors.New("yomiko already joined")
	errYomikoHasNotJoined  = errors.New("yomiko has not joined any channels")
)

const ttsSampleRate = 48000

const (
	colorSuccess = 0x26cb3f
	colorInfo    = 0x629bf8
	colorWarn    = 0xffbd32
	colorError   = 0xff5959
)

type voiceSetting struct {
	Name         string
	SpeakingRate float64
	Pitch        float64
}

func defaultVoiceSetting() *voiceSetting {
	return &voiceSetting{
		Name:         "",
		SpeakingRate: 1.0,
		Pitch:        0.0,
	}
}

type Bot struct {
	cfg      *Config
	s        *discordgo.Session
	tts      *tts.Client
	logger   *slog.Logger
	commands []*discordgo.ApplicationCommand

	mu       sync.RWMutex
	sessions map[string]*yomikoSession
	targets  map[string]string

	forVoiceSetting sync.RWMutex
	voiceSettings   map[string]*voiceSetting

	exit func()
}

func New(ctx context.Context, cfg *Config) (*Bot, error) {
	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}

	ttsOpts := []tts.ClientOption{
		tts.WithSampleRate(ttsSampleRate),
	}
	if credJSON, err := cfg.getCredentialsJSON(); err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	} else if len(credJSON) > 0 {
		ttsOpts = append(ttsOpts, tts.WithCredentialsJSON(credJSON))
	}

	c, err := tts.New(ctx, ttsOpts...)
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}

	bot := &Bot{
		cfg:           cfg,
		s:             s,
		tts:           c,
		logger:        slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})),
		sessions:      make(map[string]*yomikoSession),
		targets:       make(map[string]string),
		voiceSettings: make(map[string]*voiceSetting),
	}

	if err := bot.init(); err != nil {
		return nil, err
	}

	return bot, nil
}

func (bot *Bot) init() error {
	s := bot.s

	// Register ready as a callback for the ready events.
	s.AddHandler(bot.handleReady)

	// Register messageCreate as a callback for the messageCreate events.
	s.AddHandler(bot.handleMessageCreate)

	// Register guildCreate as a callback for the guildCreate events.
	s.AddHandler(bot.handleGuildCreate)

	s.AddHandler(bot.handleInteractionCreate)

	// We need information about guilds (which includes their channels),
	// messages and voice states.
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent | discordgo.IntentsGuildVoiceStates

	return nil
}

func (bot *Bot) Close() error {
	var errs []error

	if bot.exit != nil {
		bot.exit()
	}

	if err := bot.s.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := bot.tts.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("bot.Bot.Close: %w", errors.Join(errs...))
	}

	return nil
}

func (bot *Bot) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	bot.exit = cancel

	err := bot.s.Open()
	if err != nil {
		return fmt.Errorf("bot.Bot.Start: %w", err)
	}

	<-ctx.Done()

	return nil
}

func (bot *Bot) handleReady(s *discordgo.Session, event *discordgo.Ready) {
	bot.logger.Info("ready")
	s.UpdateGameStatus(0, "yomiko")

	commands, err := bot.getApplicationCommands(context.Background())
	if err != nil {
		bot.logger.Error("failed to get application commands", slog.Any("error", err))
		return
	}

	for _, cmd := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			bot.logger.Error("failed to create application command", slog.Any("error", err))
			continue
		}
		bot.commands = append(bot.commands, cmd)
	}
}

func (bot *Bot) handleMessageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if event.Author.ID == s.State.User.ID {
		return
	}

	guildID := event.GuildID

	bot.mu.RLock()
	defer bot.mu.RUnlock()

	ys, ok := bot.sessions[guildID]
	if !ok {
		return
	}

	if event.ChannelID != ys.TextChannelID() {
		return
	}

	var opts []tts.SynthesizeSpeechOption
	settings, ok := bot.voiceSettings[event.Author.ID]
	if ok {
		if settings.Name != "" {
			opts = append(opts, tts.WithVoiceName(settings.Name))
		}
		if settings.SpeakingRate != 1.0 {
			opts = append(opts, tts.WithSpeakingRate(settings.SpeakingRate))
		}
		if settings.Pitch != 0.0 {
			opts = append(opts, tts.WithPitch(settings.Pitch))
		}
	}

	err := ys.Read(context.Background(), contentWithMentionsReplaced(event.Message), opts...)
	if err != nil {
		bot.logger.Error("yomiko failed to read text", slog.Any("error", err))
	}
}

func contentWithMentionsReplaced(m *discordgo.Message) (content string) {
	content = m.Content

	for _, user := range m.Mentions {
		username := user.GlobalName
		if username == "" {
			username = user.Username
		}
		content = strings.NewReplacer(
			"<@"+user.ID+">", username,
			"<@!"+user.ID+">", username,
		).Replace(content)
	}
	return
}

func (bot *Bot) handleGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	bot.logger.Info("guild created", slog.String("guild_id", event.ID), slog.String("guild_name", event.Name))
}

func (bot *Bot) handleInteractionCreate(s *discordgo.Session, event *discordgo.InteractionCreate) {
	guildID := event.GuildID
	channelID := event.ChannelID

	var res *discordgo.InteractionResponse

	data := event.ApplicationCommandData()
	switch data.Name {
	case "yomiko":
		subCmd := data.Options[0]
		switch subCmd.Name {
		case "join":
			voiceChannelID := subCmd.Options[0].Value.(string)

			ys, err := bot.yomikoJoin(guildID, channelID, voiceChannelID)
			if err != nil {
				if errors.Is(err, errYomikoAlreadyJoined) {
					res = createWarnResponse("入室済です", fmt.Sprintf("読子さんは既に <#%s> に入室しています。\n<#%s> への投稿を読み上げます。", ys.VoiceChannelID(), ys.TextChannelID()))
				} else {
					res = createErrorResponse("エラーが発生しました！", "")
				}
				break
			}

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ごきげんよう、読子です",
							Description: fmt.Sprintf("読子さんは <#%s> に入室しました。", ys.VoiceChannelID()),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "leave":
			voiceChannelID, err := bot.yomikoLeave(guildID)
			if err != nil {
				if errors.Is(err, errYomikoHasNotJoined) {
					res = createWarnResponse("読子さんは入室していません", "")
				} else {
					res = createErrorResponse("エラーが発生しました！", "")
				}
				break
			}
			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "みなさま、ごきげんよう",
							Description: fmt.Sprintf("読子さんは <#%s> から退室しました。", voiceChannelID),
							Color:       colorInfo,
						},
					},
				},
			}
		case "voice":
			voiceName := subCmd.Options[0].Value.(string)

			userID := event.Member.User.ID
			bot.forVoiceSetting.Lock()
			setting, ok := bot.voiceSettings[userID]
			if !ok {
				setting = defaultVoiceSetting()
				bot.voiceSettings[userID] = setting
			}
			setting.Name = voiceName
			bot.forVoiceSetting.Unlock()

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの声を「%s」に設定しました。", voiceName),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "speed":
			speakingRate := subCmd.Options[0].Value.(float64)

			userID := event.Member.User.ID
			bot.forVoiceSetting.Lock()
			setting, ok := bot.voiceSettings[userID]
			if !ok {
				setting = defaultVoiceSetting()
				bot.voiceSettings[userID] = setting
			}
			setting.SpeakingRate = speakingRate
			bot.forVoiceSetting.Unlock()

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの読み上げ速度を「%.01f」に設定しました。", speakingRate),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "pitch":
			pitch := subCmd.Options[0].Value.(float64)

			userID := event.Member.User.ID
			bot.forVoiceSetting.Lock()
			setting, ok := bot.voiceSettings[userID]
			if !ok {
				setting = defaultVoiceSetting()
				bot.voiceSettings[userID] = setting
			}
			setting.Pitch = pitch
			bot.forVoiceSetting.Unlock()

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの声の音程を「%.01f」に設定しました。", pitch),
							Color:       colorSuccess,
						},
					},
				},
			}
		}
	}

	if res == nil {
		res = &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "コマンドを処理できませんでした",
					},
				},
			},
		}
	}
	s.InteractionRespond(event.Interaction, res)
}

func createWarnResponse(title, description string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       title,
					Description: description,
					Color:       colorWarn,
				},
			},
		},
	}
}

func createErrorResponse(title, description string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       title,
					Description: description,
					Color:       colorError,
				},
			},
		},
	}
}

func (bot *Bot) cleanupApplicationCommands() {
	for _, cmd := range bot.commands {
		err := bot.s.ApplicationCommandDelete(bot.s.State.User.ID, "", cmd.ID)
		if err != nil {
			bot.logger.Error("failed to delete application command", slog.Any("error", err))
		}
	}
}

func (bot *Bot) yomikoJoin(guildID, textChannelID, voiceChannelID string) (*yomikoSession, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	ys, ok := bot.sessions[guildID]
	if ok {
		return ys, errYomikoAlreadyJoined
	}

	ys, err := newYomikoSession(bot.s, bot.tts, guildID, textChannelID, voiceChannelID)
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.yomikoJoin: %w", err)
	}
	bot.sessions[guildID] = ys

	return ys, nil
}

func (bot *Bot) yomikoLeave(guildID string) (string, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	ys, ok := bot.sessions[guildID]
	if !ok {
		return "", errYomikoHasNotJoined
	}

	err := ys.Close()
	if err != nil {
		return "", fmt.Errorf("bot.Bot.yomikoLeave: %w", err)
	}

	delete(bot.sessions, guildID)

	return ys.VoiceChannelID(), nil
}

func makeTextChannelKey(guildID, channelID string) string {
	return strings.Join([]string{guildID, channelID}, ":")
}
