// Package bot provides a Discord bot that reads messages aloud in voice channels using text-to-speech (TTS) technology.
package bot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/kechako/yomiko/bot/internal/botutil"
	"github.com/kechako/yomiko/bot/internal/reaction"
	"github.com/kechako/yomiko/bot/internal/replacer"
	"github.com/kechako/yomiko/bot/internal/repo"
	"github.com/kechako/yomiko/bot/internal/yomiko"
	"github.com/kechako/yomiko/ssml"
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
	c        *bot.Client
	tts      *tts.Client
	repo     repo.Repository
	logger   *slog.Logger
	commands []discord.ApplicationCommand

	replacer      *replacer.Replacer
	reactions     map[snowflake.ID]*AutoReaction
	reactionChain sync.Map

	mu       sync.RWMutex
	sessions map[snowflake.ID]*yomiko.Session
	targets  map[string]string

	exit func()
}

func New(ctx context.Context, cfg *Config) (*Bot, error) {
	bot := &Bot{
		cfg:       cfg,
		logger:    initLogger(&cfg.Logging),
		replacer:  makeReplacer(cfg),
		reactions: initAutoReactions(cfg),
		sessions:  make(map[snowflake.ID]*yomiko.Session),
		targets:   make(map[string]string),
	}

	if err := bot.initTTS(ctx); err != nil {
		return nil, err
	}

	if err := bot.initRepo(ctx); err != nil {
		return nil, err
	}

	if err := bot.initDiscord(); err != nil {
		return nil, err
	}

	return bot, nil
}

func initLogger(cfg *LoggingConfig) *slog.Logger {
	var h slog.Handler
	if cfg.JSON {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: cfg.Level,
		})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: cfg.Level,
		})
	}

	return slog.New(h)
}

func (b *Bot) initDiscord() error {
	cfg := b.cfg

	c, err := disgo.New(
		cfg.Token,
		bot.WithLogger(b.logger),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
				gateway.IntentMessageContent,
				gateway.IntentGuildVoiceStates,
				gateway.IntentGuildMessageReactions,
			),
		),
		bot.WithEventListeners(
			// Register ready as a listener for the ready events.
			bot.NewListenerFunc(b.handleReady),
			// Register messageCreate as a listener for the messageCreate events.
			bot.NewListenerFunc(b.handleMessageCreate),
			// Register messageReactionAdd as a listener for the messageReactionAdd events.
			bot.NewListenerFunc(b.handleGuildMessageReactionAdd),
			// Register guildCreate as a listener for the guildCreate events.
			bot.NewListenerFunc(b.handleGuildCreate),
			// Register interactionCreate as a listener for the guildCreate events.
			bot.NewListenerFunc(b.handleInteractionCreate),
		),
		//bot.WithVoiceManagerConfigOpts(
		//	voice.WithDaveSessionCreateFunc(golibdave.NewSession),
		//),
	)
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}
	b.c = c

	commands, err := b.getApplicationCommands(context.Background())
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}

	cmds, err := b.c.Rest.SetGlobalCommands(b.c.ApplicationID, commands)
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}
	b.commands = cmds

	return nil
}

func (b *Bot) initTTS(ctx context.Context) error {
	cfg := b.cfg

	ttsOpts := []tts.ClientOption{
		tts.WithSampleRate(SampleRate),
	}
	if credJSON, err := cfg.getCredentialsJSON(); err != nil {
		return fmt.Errorf("bot.New: %w", err)
	} else if len(credJSON) > 0 {
		ttsOpts = append(ttsOpts, tts.WithCredentialsJSON(credJSON))
	}

	c, err := tts.New(ctx, ttsOpts...)
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}
	b.tts = c

	return nil
}

func (b *Bot) initRepo(ctx context.Context) error {
	r, err := repo.New(ctx, b.cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}

	b.repo = r

	return nil
}

func (b *Bot) Close() error {
	var errs []error

	if b.exit != nil {
		b.exit()
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := b.closeAllSessions(ctx); err != nil {
		errs = append(errs, err)
	}
	if b.tts != nil {
		if err := b.tts.Close(); err != nil {
			errs = append(errs, err)
		}
		b.tts = nil
	}
	if b.repo != nil {
		if err := b.repo.Close(); err != nil {
			errs = append(errs, err)
		}
		b.repo = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("bot.Bot.Close: %w", errors.Join(errs...))
	}

	return nil
}

func (b *Bot) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	b.exit = cancel

	err := b.c.OpenGateway(ctx)
	if err != nil {
		return fmt.Errorf("bot.Bot.Start: %w", err)
	}

	<-ctx.Done()

	return nil
}

func (b *Bot) handleReady(event *events.Ready) {
	b.logger.Info("ready")
	err := b.updateGameStatus(context.TODO())
	if err != nil {
		b.logger.Error("failed to update game status", slog.Any("error", err))
	}
}

func (b *Bot) updateGameStatus(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	name := fmt.Sprintf("%d 個のサーバーで読み上げ", len(b.sessions))

	err := b.c.SetPresence(ctx, gateway.WithOnlineStatus(discord.OnlineStatus(name)))
	if err != nil {
		return fmt.Errorf("bot.Bot.updateGameStatus: %w", err)
	}
	return nil
}

func (b *Bot) handleMessageCreate(event *events.MessageCreate) {
	msg := &event.Message
	if msg.Author.ID == b.c.ID() {
		return
	}

	go b.autoReaction(msg.ChannelID, msg.Author.ID, msg.ID)

	guildID := *event.GuildID

	b.mu.RLock()
	defer b.mu.RUnlock()

	ys, ok := b.sessions[guildID]
	if !ok {
		return
	}

	if event.ChannelID != ys.TextChannelID() {
		return
	}

	ctx := context.Background()

	vs, err := b.repo.GetUserVoiceSetting(ctx, msg.Author.ID)
	if err != nil {
		b.logger.Error("failed to get user voice setting", slog.Any("error", err))
		return
	}

	var opts []tts.SynthesizeSpeechOption
	if vs != nil {
		pitchSupported := true
		if voiceName := vs.VoiceName; voiceName != nil {
			opts = append(opts, tts.WithVoiceName(*voiceName))
			if isChirp3(*voiceName) {
				pitchSupported = false
			}
		}
		if vs.SpeakingRate != nil {
			opts = append(opts, tts.WithSpeakingRate(*vs.SpeakingRate))
		}
		if pitchSupported && vs.Pitch != nil {
			opts = append(opts, tts.WithPitch(*vs.Pitch))
		}
	}

	err = ys.Speak(
		context.Background(),
		b.makeSSML(msg),
		opts...)
	if err != nil {
		b.logger.Error("yomiko failed to read text", slog.Any("error", err))
	}
}

func (b *Bot) handleGuildMessageReactionAdd(event *events.GuildMessageReactionAdd) {
	if event.UserID == b.c.ID() {
		return
	}

	if name := event.Emoji.Name; name != nil {
		key := reaction.NewChainKey(event.ChannelID, event.MessageID)
		if v, ok := b.reactionChain.Load(key); ok {
			chain, ok := v.(*reaction.Chain)
			if ok {
				chain.TriggerRemove(context.Background(), *name)
			}
		}
	}
}

func isChirp3(name string) bool {
	return strings.Contains(strings.ToLower(name), "chirp3")
}

func (b *Bot) makeSSML(msg *discord.Message) string {
	root := ssml.New()
	author := messageAuthorName(msg)

	// add author
	authorSentence := &ssml.Sentence{}
	b.replacer.Replace(authorSentence, author)
	root.AddNode(&ssml.Paragraph{
		Nodes: []ssml.Node{
			authorSentence,
		},
	})

	mr := newMentionReplacer(msg)

	s := bufio.NewScanner(strings.NewReader(msg.Content))

	p := &ssml.Paragraph{}
	root.AddNode(p)

	for s.Scan() {
		sentence := &ssml.Sentence{}
		p.AddNode(sentence)

		text := mr.Replace(strings.TrimSpace(s.Text()))
		b.replacer.Replace(sentence, text)
	}

	return root.ToSSML()
}

func messageAuthorName(msg *discord.Message) (name string) {
	if msg.Member != nil {
		name = botutil.PtrToValue(msg.Member.Nick)
	}
	if name == "" {
		name = botutil.PtrToValue(msg.Author.GlobalName)
	}
	if name == "" {
		name = msg.Author.Username
	}
	name += "の発言。"

	return name
}

func newMentionReplacer(m *discord.Message) *strings.Replacer {
	var oldnew []string

	for _, user := range m.Mentions {
		username := botutil.PtrToValue(user.GlobalName)
		if username == "" {
			username = user.Username
		}
		userID := user.ID.String()
		oldnew = append(oldnew, "<@"+userID+">", username)
		oldnew = append(oldnew, "<@!"+userID+">", username)
	}

	return strings.NewReplacer(oldnew...)
}

func (b *Bot) handleGuildCreate(event *events.GuildReady) {
	guild := &event.Guild
	b.logger.Info("guild created", slog.Uint64("guild_id", uint64(guild.ID)), slog.String("guild_name", guild.Name))
}

func (b *Bot) handleInteractionCreate(event *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()

	guildID := botutil.PtrToValue(event.GuildID())
	if guildID == 0 {
		err := event.CreateMessage(*createErrorMessage("エラー", "ギルドIDを取得できませんでした。"))
		if err != nil {
			b.logger.Error("failed to create message", slog.Any("error", err))
		}
		return
	}
	channelID := event.Channel().ID()
	userID := event.Member().User.ID

	var res *discord.MessageCreate

	data := event.SlashCommandInteractionData()
	switch data.CommandName() {
	case "yomiko":
		subCmdName := botutil.PtrToValue(data.SubCommandName)
		switch subCmdName {
		case "join":
			_ = channelID
			res = b.yomikoMaintenanceMode()
			//res = b.yomikoJoinCommand(ctx, guildID, channelID, &data)
		case "leave":
			res = b.yomikoMaintenanceMode()
			//res = b.yomikoLeaveCommand(ctx, guildID)
		case "male-voice", "female-voice", "neutral-voice":
			res = b.yomikoVoiceNameCommand(ctx, userID, &data)
		case "speed":
			res = b.yomikoVoiceSpeedCommand(ctx, userID, &data)
		case "pitch":
			res = b.yomikoVoicePitchCommand(ctx, userID, &data)
		case "reset":
			res = b.yomikoVoiceResetCommand(ctx, userID)
		}
	}

	if res == nil {
		res = new(discord.NewMessageCreate().
			WithEmbeds(discord.Embed{
				Title: "コマンドを処理できませんでした",
			}))
	}
	event.CreateMessage(*res)
}

func (b *Bot) yomikoMaintenanceMode() *discord.MessageCreate {
	return createWarnMessage("ごめんなさい", "読子は現在メンテナンス中です。読み上げができません。")
}

func (b *Bot) yomikoJoinCommand(ctx context.Context, guildID, channelID snowflake.ID, data *discord.SlashCommandInteractionData) *discord.MessageCreate {
	voiceChannel, ok := data.OptChannel("voice-channel")
	if !ok {
		return createWarnMessage("エラー", "チャンネルが指定されていません。")
	}
	b.logger.Debug("yomiko join", slog.Uint64("voice-channel", uint64(voiceChannel.ID)))

	ys, err := b.yomikoJoin(ctx, guildID, channelID, voiceChannel.ID)
	if err != nil {
		if errors.Is(err, errYomikoAlreadyJoined) {
			return createWarnMessage("入室済です", fmt.Sprintf("読子さんは既に <#%s> に入室しています。\n<#%s> への投稿を読み上げます。", ys.VoiceChannelID(), ys.TextChannelID()))
		} else {
			return createErrorMessage("エラーが発生しました！", "")
		}
	}

	return createMessage(
		"ごきげんよう、読子です",
		fmt.Sprintf("読子さんは <#%s> に入室しました。", ys.VoiceChannelID()),
		colorSuccess,
	)
}

func (b *Bot) yomikoLeaveCommand(ctx context.Context, guildID snowflake.ID) *discord.MessageCreate {
	b.logger.Debug("yomiko leave", slog.Uint64("guild-id", uint64(guildID)))

	voiceChannelID, err := b.yomikoLeave(ctx, guildID)
	if err != nil {
		if errors.Is(err, errYomikoHasNotJoined) {
			return createWarnMessage("読子さんは入室していません", "")
		} else {
			return createErrorMessage("エラーが発生しました！", "")
		}
	}

	return createMessage(
		"みなさま、ごきげんよう",
		fmt.Sprintf("読子さんは <#%s> から退室しました。", voiceChannelID),
		colorInfo,
	)
}

func (b *Bot) yomikoVoiceNameCommand(ctx context.Context, userID snowflake.ID, data *discord.SlashCommandInteractionData) *discord.MessageCreate {
	voiceName, ok := data.OptString("name")
	if !ok {
		return createWarnMessage("エラー", "音声名が指定されていません。")
	}

	vs, err := b.repo.UpdateUserVoiceName(ctx, userID, voiceName)
	if err != nil {
		return createErrorMessage("エラーが発生しました！", "")
	}

	return createMessage(
		"ボイス設定",
		fmt.Sprintf("読子さんの声を「%s」に設定しました。", *vs.VoiceName),
		colorSuccess,
	)
}

func (b *Bot) yomikoVoiceSpeedCommand(ctx context.Context, userID snowflake.ID, data *discord.SlashCommandInteractionData) *discord.MessageCreate {
	speakingRate, ok := data.OptFloat("speed")
	if !ok {
		return createWarnMessage("エラー", "スピードが指定されていません。")
	}

	vs, err := b.repo.UpdateUserSpeakingRate(ctx, userID, speakingRate)
	if err != nil {
		return createErrorMessage("エラーが発生しました！", "")
	}

	return createMessage(
		"ボイス設定",
		fmt.Sprintf("読子さんの読み上げ速度を「%.01f」に設定しました。", *vs.SpeakingRate),
		colorSuccess,
	)
}

func (b *Bot) yomikoVoicePitchCommand(ctx context.Context, userID snowflake.ID, data *discord.SlashCommandInteractionData) *discord.MessageCreate {
	pitch, ok := data.OptFloat("pitch")
	if !ok {
		return createWarnMessage("エラー", "ピッチが指定されていません。")
	}

	vs, err := b.repo.UpdateUserVoicePitch(ctx, userID, pitch)
	if err != nil {
		return createErrorMessage("エラーが発生しました！", "")
	}

	return createMessage(
		"ボイス設定",
		fmt.Sprintf("読子さんの声の音程を「%.01f」に設定しました。", *vs.Pitch),
		colorSuccess,
	)
}

func (b *Bot) yomikoVoiceResetCommand(ctx context.Context, userID snowflake.ID) *discord.MessageCreate {
	_, err := b.repo.ResetUserVoiceSetting(ctx, userID)
	if err != nil {
		return createErrorMessage("エラーが発生しました！", "")
	}

	return createMessage(
		"ボイス設定",
		"読子さんの声の設定を初期値に設定しました。",
		colorSuccess,
	)
}

func createMessage(title, description string, color int) *discord.MessageCreate {
	return new(discord.NewMessageCreate().
		WithEmbeds(discord.Embed{
			Title:       title,
			Description: description,
			Color:       color,
		}))
}

func createWarnMessage(title, description string) *discord.MessageCreate {
	return createMessage(title, description, colorWarn)
}

func createErrorMessage(title, description string) *discord.MessageCreate {
	return createMessage(title, description, colorError)
}

func (b *Bot) yomikoJoin(ctx context.Context, guildID, textChannelID, voiceChannelID snowflake.ID) (*yomiko.Session, error) {
	defer b.updateGameStatus(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()

	ys, ok := b.sessions[guildID]
	if ok {
		return ys, errYomikoAlreadyJoined
	}

	ys, err := yomiko.New(ctx, &yomiko.Config{
		Bot:            b.c,
		TTS:            b.tts,
		Logger:         b.logger,
		GuildID:        guildID,
		TextChannelID:  textChannelID,
		VoiceChannelID: voiceChannelID,
	})
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.yomikoJoin: %w", err)
	}
	err = ys.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("bot.Bot.yomikoJoin: %w", err)
	}

	b.sessions[guildID] = ys

	return ys, nil
}

func (b *Bot) yomikoLeave(ctx context.Context, guildID snowflake.ID) (snowflake.ID, error) {
	defer b.updateGameStatus(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()

	ys, ok := b.sessions[guildID]
	if !ok {
		return 0, errYomikoHasNotJoined
	}

	err := ys.Close(ctx)
	if err != nil {
		return 0, fmt.Errorf("bot.Bot.yomikoLeave: %w", err)
	}

	delete(b.sessions, guildID)

	return ys.VoiceChannelID(), nil
}

func (b *Bot) autoReaction(channelID, userID, messageID snowflake.ID) {
	ar, ok := b.reactions[userID]
	if !ok {
		return
	}

	key := reaction.NewChainKey(channelID, messageID)

	chain, err := reaction.StartChain(context.Background(), &reaction.ChainConfig{
		Rest:      b.c.Rest,
		Logger:    b.logger,
		Reaction:  ar.Reaction,
		ChannelID: channelID,
		MessageID: messageID,
		Delay:     ar.MaxReactionDelay,
		Timeout:   ar.AutoRemovalTimeout,
		Completed: func() {
			b.logger.Debug("reaction chain completed",
				slog.Uint64("channel_id", uint64(channelID)),
				slog.Uint64("message_id", uint64(messageID)),
			)
			b.reactionChain.Delete(key)
		},
	})
	if err != nil {
		b.logger.Error("failed to start reaction chain", slog.Any("error", err))
	}

	b.reactionChain.Store(key, chain)
}

func (b *Bot) closeAllSessions(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var errs []error

	for guildID, ys := range b.sessions {
		err := ys.Close(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("bot.Bot.closeAllSessions: %w", err))
			continue
		}

		delete(b.sessions, guildID)
	}

	return errors.Join(errs...)
}

func makeReplacer(cfg *Config) *replacer.Replacer {
	var oldnew []string

	for _, rep := range cfg.Replacements {
		oldnew = append(oldnew, rep.From, rep.To)
	}

	return replacer.New(oldnew...)
}

func initAutoReactions(cfg *Config) map[snowflake.ID]*AutoReaction {
	reactions := make(map[snowflake.ID]*AutoReaction)
	for _, ar := range cfg.AutoReactions {
		reactions[ar.UserID] = ar
	}
	return reactions
}
