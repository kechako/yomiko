package reaction

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

type eventType int

const (
	addReaction eventType = iota
	triggerRemove
	removeAllReactions
)

type chainEvent struct {
	Type     eventType
	Reaction *Reaction
}

type ChainConfig struct {
	Rest   rest.Rest
	Logger *slog.Logger

	Reaction  *Reaction
	ChannelID snowflake.ID
	MessageID snowflake.ID

	Delay   time.Duration
	Timeout time.Duration

	Completed func()
}

func (cfg *ChainConfig) validate() error {
	if cfg.Rest == nil {
		return errors.New("rest is required")
	}
	if cfg.Logger == nil {
		return errors.New("logger is required")
	}
	if cfg.Reaction == nil {
		return errors.New("reaction is required")
	}
	if cfg.ChannelID == 0 {
		return errors.New("channel_id is required")
	}
	if cfg.MessageID == 0 {
		return errors.New("message_id is required")
	}
	if cfg.Delay < 0 {
		return errors.New("delay must be greater than or equal to 0")
	}
	if cfg.Timeout < 0 {
		return errors.New("timeout must be greater than or equal to 0")
	}
	return nil
}

type Chain struct {
	rest   rest.Rest
	logger *slog.Logger

	reaction  *Reaction
	channelID snowflake.ID
	messageID snowflake.ID
	current   *Reaction

	delay   time.Duration
	timeout time.Duration

	completed func()

	events chan *chainEvent
	exit   func()
}

func StartChain(ctx context.Context, cfg *ChainConfig) (*Chain, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	c := &Chain{
		rest:   cfg.Rest,
		logger: cfg.Logger,

		reaction:  cfg.Reaction,
		channelID: cfg.ChannelID,
		messageID: cfg.MessageID,

		delay:   randomDelay(cfg.Delay),
		timeout: cfg.Timeout,

		completed: cfg.Completed,

		events: make(chan *chainEvent, 16),
	}
	c.start(ctx)

	return c, nil
}

func (c *Chain) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.exit = cancel

	go c.loop(ctx)
}

func (c *Chain) TriggerRemove(ctx context.Context, emoji string) {
	c.dispatch(ctx, &chainEvent{
		Type: triggerRemove,
		Reaction: &Reaction{
			Emoji: emoji,
		},
	})
}

func (c *Chain) loop(ctx context.Context) {
	defer func() {
		if c.completed != nil {
			c.completed()
		}
	}()
	defer c.exit()

	c.scheduleFirstReaction(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-c.events:
			switch event.Type {
			case addReaction:
				c.addReaction(ctx, event.Reaction)
				if c.current == nil {
					c.scheduleAutoRemove(ctx)
				}
				c.current = event.Reaction
			case triggerRemove:
				c.tryRemoveReaction(ctx, event.Reaction.Emoji)
			case removeAllReactions:
				c.removeAllReactions(ctx)
				return
			}
		}
	}
}

func (c *Chain) dispatch(ctx context.Context, event *chainEvent) {
	select {
	case <-ctx.Done():
	case c.events <- event:
	}
}

func (c *Chain) scheduleFirstReaction(ctx context.Context) {
	go func() {
		t := time.NewTimer(c.delay)
		defer t.Stop()

		event := &chainEvent{
			Type:     addReaction,
			Reaction: c.reaction,
		}

		select {
		case <-t.C:
			c.dispatch(ctx, event)
		case <-ctx.Done():
		}
	}()
}

func (c *Chain) scheduleAutoRemove(ctx context.Context) {
	if c.timeout == 0 {
		return
	}

	go func() {
		t := time.NewTimer(c.timeout)
		defer t.Stop()

		event := &chainEvent{
			Type: removeAllReactions,
		}

		select {
		case <-t.C:
			c.dispatch(ctx, event)
		case <-ctx.Done():
		}
	}()
}

func (c *Chain) addReaction(ctx context.Context, reaction *Reaction) {
	c.logger.Debug("adding reaction", slog.String("emoji", reaction.Emoji))

	err := c.rest.AddReaction(c.channelID, c.messageID, reaction.Emoji, rest.WithCtx(ctx))
	if err != nil {
		c.logger.Error("failed to add reaction",
			slog.Any("error", err),
			slog.Uint64("channel_id", uint64(c.channelID)),
			slog.Uint64("message_id", uint64(c.messageID)),
			slog.String("emoji", reaction.Emoji))
	}
}

func (c *Chain) tryRemoveReaction(ctx context.Context, emoji string) {
	c.logger.Debug("trying to remove reaction", slog.String("emoji", emoji))

	current := c.current
	if current == nil {
		c.logger.Debug("current reaction is nil, ignoring trigger remove")
		return
	}

	if emoji != current.RemovalEmoji {
		c.logger.Debug("emoji does not match current reaction removal emoji, ignoring trigger remove")
		return
	}

	var next *Reaction
	if current.Next != nil && current.ProbabilityNext > 0 {
		if rand.IntN(int(1/current.ProbabilityNext)) == 0 {
			next = current.Next
		}
	}

	var event *chainEvent
	if next == nil {
		c.logger.Debug("no next reaction, scheduling remove all reactions")
		event = &chainEvent{
			Type: removeAllReactions,
		}
	} else {
		c.logger.Debug("next reaction exists, scheduling add next reaction",
			slog.String("next_reaction", next.Emoji),
		)
		event = &chainEvent{
			Type:     addReaction,
			Reaction: next,
		}
	}

	c.dispatch(ctx, event)
}

func (c *Chain) removeAllReactions(ctx context.Context) {
	c.logger.Debug("removing all reactions")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for r := c.reaction; r != nil; r = r.Next {
		c.removeReaction(ctx, r.Emoji)
		if r == c.current {
			break
		}
	}
}

func (c *Chain) removeReaction(ctx context.Context, emoji string) {
	err := c.rest.RemoveOwnReaction(c.channelID, c.messageID, emoji, rest.WithCtx(ctx))
	if err != nil {
		c.logger.Error("failed to remove reaction",
			slog.Any("error", err),
			slog.Uint64("channel_id", uint64(c.channelID)),
			slog.Uint64("message_id", uint64(c.messageID)),
			slog.String("emoji", emoji))
	}
}

func randomDelay(delay time.Duration) time.Duration {
	if delay < time.Second {
		return 0
	}
	ds := delay / time.Second
	d := time.Duration(rand.Int64N(int64(ds)))
	return d * time.Second
}

type ChainKey [2]snowflake.ID

func NewChainKey(channelID, messageID snowflake.ID) ChainKey {
	return ChainKey{
		channelID,
		messageID,
	}
}
