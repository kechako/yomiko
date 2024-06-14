package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/kechako/yomiko/audio/pcm"
	"github.com/kechako/yomiko/tts"
	"gopkg.in/hraban/opus.v2"
)

const frameSizeMs = 20
const frameSize = SampleRate * frameSizeMs / 1000

type yomikoSession struct {
	s              *discordgo.Session
	conn           *discordgo.VoiceConnection
	tts            *tts.Client
	enc            *opus.Encoder
	guildID        string
	textChannelID  string
	voiceChannelID string
}

func newYomikoSession(s *discordgo.Session, ttsClient *tts.Client, guildID, textChannelID, voiceChannelID string) (*yomikoSession, error) {
	enc, err := opus.NewEncoder(SampleRate, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("bot.newYomikoSession: %w", err)
	}

	conn, err := s.ChannelVoiceJoin(guildID, voiceChannelID, false, true)
	if err != nil {
		return nil, fmt.Errorf("bot.newYomikoSession: %w", err)
	}

	return &yomikoSession{
		s:              s,
		conn:           conn,
		tts:            ttsClient,
		enc:            enc,
		guildID:        guildID,
		textChannelID:  textChannelID,
		voiceChannelID: voiceChannelID,
	}, nil
}

func (s *yomikoSession) Close() error {
	return s.conn.Disconnect()
}

func (s *yomikoSession) GuildID() string {
	return s.guildID
}

func (s *yomikoSession) TextChannelID() string {
	return s.textChannelID
}

func (s *yomikoSession) VoiceChannelID() string {
	return s.voiceChannelID
}

func (s *yomikoSession) Read(ctx context.Context, text string, opts ...tts.SynthesizeSpeechOption) error {
	p, err := s.tts.SynthesizeSpeech(ctx, text, opts...)
	if err != nil {
		return fmt.Errorf("bot.yomikoSession.Read: %w", err)
	}

	s.conn.Speaking(true)

	err = s.splitFrames(p, func(data []int16) error {
		var buf [1276]byte
		n, err := s.enc.Encode(data, buf[:])
		if err != nil {
			return fmt.Errorf("bot.yomikoSession.Read: %w", err)
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

func (s *yomikoSession) splitFrames(p []byte, f func(data []int16) error) error {
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

func fillZero(data []int16) {
	for i := range data {
		data[i] = 0
	}
}
