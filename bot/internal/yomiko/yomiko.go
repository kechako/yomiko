package yomiko

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/kechako/yomiko/audio/buffer"
	"github.com/kechako/yomiko/audio/pcm"
	"github.com/kechako/yomiko/tts"
	"gopkg.in/hraban/opus.v2"
)

const frameSizeMs = 20

type Config struct {
	Session        *discordgo.Session
	TTS            *tts.Client
	GuildID        string
	TextChannelID  string
	VoiceChannelID string
	SampleRate     int
}

type Session struct {
	s    *discordgo.Session
	conn *discordgo.VoiceConnection
	mu   sync.Mutex

	tts *tts.Client
	enc *opus.Encoder

	frameSize int
	framePool *buffer.Pool

	guildID        string
	textChannelID  string
	voiceChannelID string
}

func New(cfg *Config) (*Session, error) {
	s := cfg.Session

	enc, err := opus.NewEncoder(cfg.SampleRate, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("yomiko.New: %w", err)
	}

	conn, err := s.ChannelVoiceJoin(cfg.GuildID, cfg.VoiceChannelID, false, true)
	if err != nil {
		return nil, fmt.Errorf("yomiko.New: %w", err)
	}

	frameSize := cfg.SampleRate * frameSizeMs / 1000

	return &Session{
		s:              s,
		conn:           conn,
		tts:            cfg.TTS,
		enc:            enc,
		frameSize:      frameSize,
		framePool:      buffer.NewPool(frameSize),
		guildID:        cfg.GuildID,
		textChannelID:  cfg.TextChannelID,
		voiceChannelID: cfg.VoiceChannelID,
	}, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}

	err := s.conn.Disconnect()
	if err != nil {
		return err
	}
	s.conn = nil

	return nil
}

func (s *Session) GuildID() string {
	return s.guildID
}

func (s *Session) TextChannelID() string {
	return s.textChannelID
}

func (s *Session) VoiceChannelID() string {
	return s.voiceChannelID
}

func (s *Session) Read(ctx context.Context, ssml string, opts ...tts.SynthesizeSpeechOption) error {
	p, err := s.tts.SynthesizeSpeech(ctx, ssml, opts...)
	if err != nil {
		return fmt.Errorf("yomiko.Session.Read: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}

	s.conn.Speaking(true)

	err = s.splitFrames(p, func(data []int16) error {
		var buf [1276]byte
		n, err := s.enc.Encode(data, buf[:])
		if err != nil {
			return fmt.Errorf("yomiko.Session.Read: %w", err)
		}

		s.conn.OpusSend <- buf[:n]

		return nil
	})
	if err != nil {
		return err
	}

	s.conn.Speaking(false)

	return nil
}

func (s *Session) splitFrames(p []byte, f func(data []int16) error) error {
	data := s.framePool.Get()

	n := pcm.SamplesToBytes[int16](s.frameSize)

	for i := 0; i < len(p); i += n {
		tail := min(len(p), i+n)
		n, err := pcm.Decode(*data, p[i:tail], pcm.LittleEndian)
		if err != nil {
			return err
		}
		copied := pcm.BytesToSamples[int16](n)
		if copied < len(*data) {
			fillZero((*data)[copied:])
		}

		err = f(*data)
		if err != nil {
			return err
		}
	}

	return nil
}

func fillZero(data []int16) {
	for i := range data {
		data[i] = 0
	}
}
