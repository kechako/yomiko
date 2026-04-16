// Package repo defines the repository layer for databases.
package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/disgoorg/snowflake/v2"
	"github.com/kechako/yomiko/ent"
	"github.com/kechako/yomiko/ent/voicesetting"
)

type Repository interface {
	io.Closer
	UpdateUserVoiceName(ctx context.Context, userID snowflake.ID, voiceName string) (*ent.VoiceSetting, error)
	UpdateUserSpeakingRate(ctx context.Context, userID snowflake.ID, speakingRate float64) (*ent.VoiceSetting, error)
	UpdateUserVoicePitch(ctx context.Context, userID snowflake.ID, pitch float64) (*ent.VoiceSetting, error)
	ResetUserVoiceSetting(ctx context.Context, userID snowflake.ID) (*ent.VoiceSetting, error)
	GetUserVoiceSetting(ctx context.Context, userID snowflake.ID) (*ent.VoiceSetting, error)
}

type repository struct {
	c *ent.Client
}

func New(ctx context.Context, path string) (Repository, error) {
	c, err := ent.Open("sqlite3", makeDataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("repo.New: failed to open database: %s: %w", path, err)
	}

	if err := c.Schema.Create(ctx); err != nil {
		return nil, fmt.Errorf("repo.New: failed to migrate database: %w", err)
	}

	return &repository{c}, nil
}

func (r *repository) Close() error {
	return r.c.Close()
}

func (r *repository) UpdateUserVoiceName(ctx context.Context, userID snowflake.ID, voiceName string) (*ent.VoiceSetting, error) {
	return r.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetVoiceName(voiceName)
	})
}

func (r *repository) UpdateUserSpeakingRate(ctx context.Context, userID snowflake.ID, speakingRate float64) (*ent.VoiceSetting, error) {
	return r.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetSpeakingRate(speakingRate)
	})
}

func (r *repository) UpdateUserVoicePitch(ctx context.Context, userID snowflake.ID, pitch float64) (*ent.VoiceSetting, error) {
	return r.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.SetPitch(pitch)
	})
}

func (r *repository) ResetUserVoiceSetting(ctx context.Context, userID snowflake.ID) (*ent.VoiceSetting, error) {
	return r.updateUserVoiceSetting(ctx, userID, func(m *ent.VoiceSettingMutation) {
		m.ClearVoiceName()
		m.ClearSpeakingRate()
		m.ClearPitch()
	})
}

func (r *repository) updateUserVoiceSetting(ctx context.Context, userID snowflake.ID, f func(m *ent.VoiceSettingMutation)) (*ent.VoiceSetting, error) {
	tx, err := r.c.BeginTx(ctx, nil)
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

func (r *repository) GetUserVoiceSetting(ctx context.Context, userID snowflake.ID) (*ent.VoiceSetting, error) {
	vs, err := r.c.VoiceSetting.Query().
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

func makeDataSourceName(path string) string {
	opts := url.Values{}
	opts.Set("mode", "rwc")
	opts.Set("_fk", "1")

	n := &url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: opts.Encode(),
	}

	return n.String()
}
