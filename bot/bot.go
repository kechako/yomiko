package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/kechako/yomiko/ent"
	"github.com/kechako/yomiko/ent/voicesetting"
	"github.com/kechako/yomiko/tts"
	_ "github.com/mattn/go-sqlite3"
)

var (
	errYomikoAlreadyJoined = errors.New("yomiko already joined")
	errYomikoHasNotJoined  = errors.New("yomiko has not joined any channels")
)

const SampleRate = 48000

const (
	colorSuccess = 0x26cb3f
	colorInfo    = 0x629bf8
	colorWarn    = 0xffbd32
	colorError   = 0xff5959
)

type Bot struct {
	cfg      *Config
	s        *discordgo.Session
	tts      *tts.Client
	ent      *ent.Client
	logger   *slog.Logger
	commands []*discordgo.ApplicationCommand

	mu       sync.RWMutex
	sessions map[string]*yomikoSession
	targets  map[string]string

	exit func()
}

func New(ctx context.Context, cfg *Config) (*Bot, error) {
	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}

	ttsOpts := []tts.ClientOption{
		tts.WithSampleRate(SampleRate),
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

	e, err := ent.Open("sqlite3", makeDataSourceName(cfg))
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}

	if err := e.Schema.Create(ctx); err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}

	bot := &Bot{
		cfg:      cfg,
		s:        s,
		tts:      c,
		ent:      e,
		logger:   slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})),
		sessions: make(map[string]*yomikoSession),
		targets:  make(map[string]string),
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

func makeDataSourceName(cfg *Config) string {
	opts := url.Values{}
	opts.Set("mode", "rwc")
	opts.Set("_fk", "1")

	n := &url.URL{
		Scheme:   "file",
		Path:     cfg.DatabasePath,
		RawQuery: opts.Encode(),
	}

	return n.String()
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
	if err := bot.ent.Close(); err != nil {
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

	ctx := context.Background()

	vs, err := bot.getUserVoiceSetting(ctx, event.Author.ID)
	if err != nil {
		bot.logger.Error("failed to get user voice setting", slog.Any("error", err))
		return
	}

	var opts []tts.SynthesizeSpeechOption
	if vs != nil {
		if vs.VoiceName != nil {
			opts = append(opts, tts.WithVoiceName(*vs.VoiceName))
		}
		if vs.SpeakingRate != nil {
			opts = append(opts, tts.WithSpeakingRate(*vs.SpeakingRate))
		}
		if vs.Pitch != nil {
			opts = append(opts, tts.WithPitch(*vs.Pitch))
		}
	}

	err = ys.Read(
		context.Background(),
		fmt.Sprintf(
			"%s: %s",
			event.Author.GlobalName,
			contentWithMentionsReplaced(event.Message),
		), opts...)
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
	ctx := context.Background()

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
			vs, err := bot.updateUserVoiceName(ctx, userID, voiceName)
			if err != nil {
				res = createErrorResponse("エラーが発生しました！", "")
				break
			}

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの声を「%s」に設定しました。", *vs.VoiceName),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "speed":
			speakingRate := subCmd.Options[0].Value.(float64)

			userID := event.Member.User.ID
			vs, err := bot.updateUserSpeakingRate(ctx, userID, speakingRate)
			if err != nil {
				res = createErrorResponse("エラーが発生しました！", "")
				break
			}

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの読み上げ速度を「%.01f」に設定しました。", *vs.SpeakingRate),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "pitch":
			pitch := subCmd.Options[0].Value.(float64)

			userID := event.Member.User.ID
			vs, err := bot.updateUserVoicePitch(ctx, userID, pitch)
			if err != nil {
				res = createErrorResponse("エラーが発生しました！", "")
				break
			}

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: fmt.Sprintf("読子さんの声の音程を「%.01f」に設定しました。", *vs.Pitch),
							Color:       colorSuccess,
						},
					},
				},
			}
		case "reset":
			userID := event.Member.User.ID
			_, err := bot.resetUserVoiceSetting(ctx, userID)
			if err != nil {
				res = createErrorResponse("エラーが発生しました！", "")
				break
			}

			res = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "ボイス設定",
							Description: "読子さんの声の設定を初期値に設定しました。",
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

func (bot *Bot) updateUserVoiceName(ctx context.Context, userID, voiceName string) (*ent.VoiceSetting, error) {
	return bot.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetVoiceName(voiceName)
	})
}

func (bot *Bot) updateUserSpeakingRate(ctx context.Context, userID string, speakingRate float64) (*ent.VoiceSetting, error) {
	return bot.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetSpeakingRate(speakingRate)
	})
}

func (bot *Bot) updateUserVoicePitch(ctx context.Context, userID string, pitch float64) (*ent.VoiceSetting, error) {
	return bot.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetPitch(pitch)
	})
}

func (bot *Bot) resetUserVoiceSetting(ctx context.Context, userID string) (*ent.VoiceSetting, error) {
	return bot.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.ClearVoiceName()
		m.ClearSpeakingRate()
		m.ClearPitch()
	})
}

func (bot *Bot) updateUserVoiceSetting(ctx context.Context, userID string, f func(m *ent.VoiceSettingMutation)) (*ent.VoiceSetting, error) {
	tx, err := bot.ent.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.getVoiceSetting: %w", err)
	}
	vs, err := tx.VoiceSetting.Query().
		Where(voicesetting.UserID(userID)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, rollback(tx, fmt.Errorf("bot.Bot.getVoiceSetting: %w", err))
	}

	if vs == nil {
		// create
		create := tx.VoiceSetting.Create().
			SetUserID(userID)

		f(create.Mutation())

		vs, err = create.Save(ctx)
		if err != nil {
			return nil, rollback(tx, fmt.Errorf("bot.Bot.getVoiceSetting: %w", err))
		}
	} else {
		// update
		update := tx.VoiceSetting.UpdateOne(vs)
		f(update.Mutation())

		vs, err = update.Save(ctx)
		if err != nil {
			return nil, rollback(tx, fmt.Errorf("bot.Bot.getVoiceSetting: %w", err))
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return vs, nil
}

func (bot *Bot) getUserVoiceSetting(ctx context.Context, userID string) (*ent.VoiceSetting, error) {
	vs, err := bot.ent.VoiceSetting.Query().
		Where(voicesetting.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("bot.Bot.getUserVoiceSetting: %w", err)
	}

	return vs, nil
}

func rollback(tx *ent.Tx, err error) error {
	if rerr := tx.Rollback(); rerr != nil {
		err = errors.Join(err, rerr)
	}
	return err
}
