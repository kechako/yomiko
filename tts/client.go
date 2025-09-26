package tts

import (
	"context"
	"errors"
	"fmt"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"google.golang.org/api/option"
)

const (
	DefaultLanguageCode = "ja-JP"
	DefaultSampleRate   = 48000

	DefaultVoiceName    = ""
	DefaultSpeakingRate = 1.0
	DefaultPitch        = 0.0

	MaxSpeakingRate = 4.0
	MinSpeakingRate = 0.25
	MaxPitch        = 20.0
	MinPitch        = -20.0
)

type InputMode int

const (
	Text InputMode = iota
	SSML
)

type CustomPronunciation struct {
	Phrase        string
	Pronunciation string
}

type Client struct {
	opts   *clientOptions
	client *texttospeech.Client
}

func New(ctx context.Context, opts ...ClientOption) (*Client, error) {
	options := clientOptions{
		languageCode: DefaultLanguageCode,
		sampleRate:   DefaultSampleRate,
	}
	for _, opt := range opts {
		opt.apply(&options)
	}

	var clientOpts []option.ClientOption
	if len(options.credentialsJSON) > 0 {
		clientOpts = append(clientOpts, option.WithCredentialsJSON(options.credentialsJSON))
	}

	client, err := texttospeech.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("tts.New: %w", err)
	}
	return &Client{
		opts:   &options,
		client: client,
	}, nil
}

type Voice = texttospeechpb.Voice

var (
	GenderMale    = texttospeechpb.SsmlVoiceGender_MALE
	GenderFemale  = texttospeechpb.SsmlVoiceGender_FEMALE
	GenderNeutral = texttospeechpb.SsmlVoiceGender_NEUTRAL
)

func (c *Client) ListVoices(ctx context.Context) ([]*Voice, error) {
	res, err := c.client.ListVoices(ctx, &texttospeechpb.ListVoicesRequest{
		LanguageCode: c.opts.languageCode,
	})
	if err != nil {
		return nil, fmt.Errorf("tts.Client.SynthesizeSpeech: %w", err)
	}

	return res.GetVoices(), nil
}

func (c *Client) SynthesizeSpeech(ctx context.Context, input string, opts ...SynthesizeSpeechOption) ([]byte, error) {
	o := synthesizeSpeechOptions{
		speakingRate: 1.0,
		pitch:        0.0,
	}
	for _, opt := range opts {
		opt.apply(&o)
	}

	var customPronunciations *texttospeechpb.CustomPronunciations
	if len(o.customPronunciations) > 0 {
		customPronunciations = &texttospeechpb.CustomPronunciations{}
		for _, cp := range o.customPronunciations {
			customPronunciations.Pronunciations = append(
				customPronunciations.Pronunciations,
				makeCustomPronunciationParams(cp),
			)
		}
	}

	si := &texttospeechpb.SynthesisInput{
		CustomPronunciations: customPronunciations,
	}
	switch o.inputMode {
	case Text:
		si.InputSource = &texttospeechpb.SynthesisInput_Text{
			Text: input,
		}
	case SSML:
		si.InputSource = &texttospeechpb.SynthesisInput_Ssml{
			Ssml: input,
		}
	default:
		return nil, errors.New("invalid input mode")
	}

	res, err := c.client.SynthesizeSpeech(ctx, &texttospeechpb.SynthesizeSpeechRequest{
		Input: si,
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: c.opts.languageCode,
			Name:         o.voiceName,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16,
			SampleRateHertz: int32(c.opts.sampleRate),
			SpeakingRate:    o.speakingRate,
			Pitch:           o.pitch,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("tts.Client.SynthesizeSpeech: %w", err)
	}

	return res.GetAudioContent(), nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func makeCustomPronunciationParams(cp *CustomPronunciation) *texttospeechpb.CustomPronunciationParams {
	encoding := texttospeechpb.CustomPronunciationParams_PHONETIC_ENCODING_JAPANESE_YOMIGANA
	return &texttospeechpb.CustomPronunciationParams{
		Phrase:           &cp.Phrase,
		PhoneticEncoding: &encoding,
		Pronunciation:    &cp.Pronunciation,
	}
}

type clientOptions struct {
	credentialsJSON []byte
	languageCode    string
	sampleRate      int
}

type ClientOption interface {
	apply(opts *clientOptions)
}

func WithCredentialsJSON(p []byte) ClientOption {
	return withCredentialsJSON(p)
}

type withCredentialsJSON []byte

func (w withCredentialsJSON) apply(o *clientOptions) {
	o.credentialsJSON = make([]byte, len(w))
	copy(o.credentialsJSON, w)
}

type withLanguageCode string

func (w withLanguageCode) apply(o *clientOptions) {
	o.languageCode = string(w)
}

func WithLanguageCode(code string) ClientOption {
	return withLanguageCode(code)
}

type withSampleRate int

func (w withSampleRate) apply(o *clientOptions) {
	o.sampleRate = int(w)
}

func WithSampleRate(sampleRate int) ClientOption {
	return withSampleRate(sampleRate)
}

type synthesizeSpeechOptions struct {
	inputMode            InputMode
	voiceName            string
	speakingRate         float64
	pitch                float64
	customPronunciations []*CustomPronunciation
}

type SynthesizeSpeechOption interface {
	apply(opts *synthesizeSpeechOptions)
}

type withInputMode InputMode

func (w withInputMode) apply(o *synthesizeSpeechOptions) {
	o.inputMode = InputMode(w)
}

func WithInputMode(mode InputMode) SynthesizeSpeechOption {
	return withInputMode(mode)
}

type withVoiceName string

func (w withVoiceName) apply(o *synthesizeSpeechOptions) {
	o.voiceName = string(w)
}

func WithVoiceName(name string) SynthesizeSpeechOption {
	return withVoiceName(name)
}

type withSpeakingRate float64

func (w withSpeakingRate) apply(o *synthesizeSpeechOptions) {
	o.speakingRate = float64(w)
}

func WithSpeakingRate(speakingRate float64) SynthesizeSpeechOption {
	return withSpeakingRate(speakingRate)
}

type withPitch float64

func (w withPitch) apply(o *synthesizeSpeechOptions) {
	o.pitch = float64(w)
}

func WithPitch(pitch float64) SynthesizeSpeechOption {
	return withPitch(pitch)
}

type withCustomPronunciations struct {
	cps []*CustomPronunciation
}

func (w withCustomPronunciations) apply(o *synthesizeSpeechOptions) {
	o.customPronunciations = w.cps
}

func WithCustomPronunciations(cps []*CustomPronunciation) SynthesizeSpeechOption {
	return withCustomPronunciations{cps}
}
