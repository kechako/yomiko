// Package yomiko provides a text-to-speech (TTS) session for Discord voice channels.
package yomiko

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/kechako/yomiko/audio/pcm"
	"github.com/kechako/yomiko/tts"
	"gopkg.in/hraban/opus.v2"
)

const SampleRate = 48000
const frameSizeMs = 20
const frameSize = SampleRate * frameSizeMs / 1000
const bufferSize = 1276

type speachParam struct {
	SSML    string
	Options []tts.SynthesizeSpeechOption
}

type Session struct {
	bot  *bot.Client
	conn voice.Conn

	logger *slog.Logger

	tts *tts.Client
	enc *opus.Encoder

	speachCh chan speachParam
	encodeCh chan []byte
	bufPool  sync.Pool
	writeCh  chan []byte

	guildID        snowflake.ID
	textChannelID  snowflake.ID
	voiceChannelID snowflake.ID

	wg   sync.WaitGroup
	exit func()
}

type Config struct {
	Bot            *bot.Client
	TTS            *tts.Client
	Logger         *slog.Logger
	GuildID        snowflake.ID
	TextChannelID  snowflake.ID
	VoiceChannelID snowflake.ID
}

func New(ctx context.Context, cfg *Config) (*Session, error) {
	enc, err := opus.NewEncoder(SampleRate, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("bot.newYomikoSession: %w", err)
	}

	conn := cfg.Bot.VoiceManager.CreateConn(cfg.GuildID)

	logger := cfg.Logger.With(
		slog.Uint64("guild_id", uint64(cfg.GuildID)),
		slog.Uint64("text_channel_id", uint64(cfg.TextChannelID)),
		slog.Uint64("voice_channel_id", uint64(cfg.VoiceChannelID)),
	)

	return &Session{
		bot:            cfg.Bot,
		conn:           conn,
		logger:         logger,
		tts:            cfg.TTS,
		enc:            enc,
		speachCh:       make(chan speachParam),
		encodeCh:       make(chan []byte),
		writeCh:        make(chan []byte),
		guildID:        cfg.GuildID,
		textChannelID:  cfg.TextChannelID,
		voiceChannelID: cfg.VoiceChannelID,
	}, nil
}

func (s *Session) Close(ctx context.Context) error {
	if s.conn == nil {
		return nil
	}

	if s.exit != nil {
		s.exit()
	}

	s.conn.Close(ctx)
	s.conn = nil

	s.wg.Wait()

	return nil
}

func (s *Session) Open(ctx context.Context) error {
	wkCtx, cancel := context.WithCancel(context.Background())
	s.exit = cancel

	err := s.conn.Open(ctx, s.voiceChannelID, false, false)
	if err != nil {
		return fmt.Errorf("yomiko.Open: %w", err)
	}
	err = s.conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone)
	if err != nil {
		s.logger.Error("error setting speaking flag", slog.Any("error", err))
		return fmt.Errorf("yomiko.Open: %w", err)
	}

	s.wg.Go(func() {
		s.read(wkCtx)
	})
	s.wg.Go(func() {
		s.write(wkCtx)
	})

	return nil
}

func (s *Session) read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, err := s.conn.UDP().ReadPacket(); err != nil {
			s.logger.Error("error reading udp packet", slog.Any("error", err))
		}
	}
}

func (s *Session) speach(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case param := <-s.speachCh:
			p, err := s.tts.SynthesizeSpeech(ctx, param.SSML, param.Options...)
			if err != nil {
				s.logger.Error("error synthesizing speech", slog.Any("error", err))
				break
			}
			s.encodeCh <- p
		}
	}
}

func (s *Session) encode(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-s.encodeCh:
			err := s.splitFrames(p, func(data []int16) error {
				buf := s.getBuf()
				n, err := s.enc.Encode(data, buf)
				if err != nil {
					return fmt.Errorf("yomiko.Session.encode: %w", err)
				}

				select {
				case <-ctx.Done():
					return nil
				case s.writeCh <- buf[:n]:
				}

				return nil
			})
			if err != nil {
				s.logger.Error("error encoding pcm data", slog.Any("error", err))
			}
		}
	}
}

func (s *Session) write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-s.writeCh:
			_, err := s.conn.UDP().Write(buf)
			s.putBuf(buf)
			if err != nil {
				s.logger.Error("error writing udp packet", slog.Any("error", err))
			}
		}
	}
}

func (s *Session) GuildID() snowflake.ID {
	return s.guildID
}

func (s *Session) TextChannelID() snowflake.ID {
	return s.textChannelID
}

func (s *Session) VoiceChannelID() snowflake.ID {
	return s.voiceChannelID
}

func (s *Session) Speak(ctx context.Context, ssml string, opts ...tts.SynthesizeSpeechOption) error {
	p := speachParam{
		SSML:    ssml,
		Options: opts,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.speachCh <- p:
	}

	//err = s.conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone)
	//if err != nil {
	//	return fmt.Errorf("yomiko.Speak: %w", err)
	//}

	//err = s.conn.SetSpeaking(ctx, voice.SpeakingFlagNone)
	//if err != nil {
	//	return fmt.Errorf("yomiko.Speak: %w", err)
	//}

	return nil
}

func (s *Session) splitFrames(p []byte, f func(data []int16) error) error {
	var data [frameSize]int16

	n := pcm.SamplesToBytes[int16](frameSize)

	for i := 0; i < len(p); i += n {
		tail := min(len(p), i+n)
		n, err := pcm.Decode(data[:], p[i:tail], pcm.LittleEndian)
		if err != nil {
			return err
		}
		copied := pcm.BytesToSamples[int16](n)
		if copied < len(data) {
			fillZero(data[copied:])
		}

		err = f(data[:])
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) getBuf() []byte {
	v := s.bufPool.Get()
	if v == nil {
		return make([]byte, bufferSize)
	}
	buf, ok := v.(*[]byte)
	if !ok {
		return make([]byte, bufferSize)
	}
	return *buf
}

func (s *Session) putBuf(b []byte) {
	b = b[:cap(b)]
	s.bufPool.Put(&b)
}

func fillZero(data []int16) {
	for i := range data {
		data[i] = 0
	}
}
