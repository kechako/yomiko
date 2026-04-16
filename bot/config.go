package bot

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/disgoorg/snowflake/v2"
	"github.com/kechako/yomiko/bot/internal/reaction"
)

type Replacement struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

type LoggingConfig struct {
	Level slog.Level `toml:"level"`
	JSON  bool       `toml:"json"`
}

type AutoReaction struct {
	UserID             snowflake.ID       `toml:"user_id"`
	MaxReactionDelay   time.Duration      `toml:"max_reaction_delay"`
	AutoRemovalTimeout time.Duration      `toml:"auto_removal_timeout"`
	Reaction           *reaction.Reaction `toml:"reaction"`
}

func (ar *AutoReaction) validate() error {
	if ar.UserID == 0 {
		return errors.New("user_id is required")
	}
	if ar.MaxReactionDelay < 0 {
		return errors.New("max_reaction_delay must be greater than or equal to 0")
	}
	if ar.AutoRemovalTimeout <= 0 {
		return errors.New("auto_removal_timeout must be greater than 0")
	}
	if ar.Reaction == nil {
		return errors.New("reaction is required")
	}
	if ar.Reaction.Emoji == "" {
		return errors.New("reaction.emoji is required")
	}
	if ar.Reaction.RemovalEmoji == "" {
		return errors.New("reaction.removal_emoji is required")
	}
	if ar.Reaction.ProbabilityNext < 0 || ar.Reaction.ProbabilityNext > 1 {
		return errors.New("reaction.probability_next must be between 0 and 1")
	}
	return nil
}

type Config struct {
	Token           string          `toml:"token"`
	CredentialsJSON string          `toml:"credentials_json"`
	CredentialsFile string          `toml:"credentials_file"`
	DatabasePath    string          `toml:"database_path"`
	Logging         LoggingConfig   `toml:"logging"`
	Replacements    []*Replacement  `toml:"replacements"`
	AutoReactions   []*AutoReaction `toml:"auto_reactions"`
}

func (cfg *Config) validate() error {
	if cfg.Token == "" {
		return errors.New("token is required")
	}
	if cfg.CredentialsJSON != "" && cfg.CredentialsFile != "" {
		return errors.New("credentials_json and credentials_file cannot be set at the same time")
	}
	if cfg.CredentialsJSON == "" && cfg.CredentialsFile == "" {
		return errors.New("either credentials_json or credentials_file must be set")
	}
	if cfg.DatabasePath == "" {
		return errors.New("database_path is required")
	}

	for i, ar := range cfg.AutoReactions {
		if err := ar.validate(); err != nil {
			return fmt.Errorf("auto_reactions[%d]: %w", i, err)
		}
	}

	return nil
}

func ReadConfigFile(name string) (*Config, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("bot.ReadConfigFile: %w", err)
	}
	defer file.Close()

	return ReadConfig(file)
}

func ReadConfig(r io.Reader) (*Config, error) {
	// TOML 内に埋め込まれた環境変数 ${XXXXX} を展開する
	var buf bytes.Buffer
	s := bufio.NewScanner(r)
	for s.Scan() {
		buf.WriteString(os.ExpandEnv(s.Text()))
		buf.WriteByte('\n')
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("bot.ReadConfig: %w", err)
	}

	cfg := new(Config)
	_, err := toml.NewDecoder(&buf).Decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("bot.ReadConfig: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("bot.ReadConfig: %w", err)
	}

	return cfg, nil
}

func WriteConfigFile(name string, cfg *Config) error {
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("bot.WriteConfigFile: %w", err)
	}
	defer file.Close()

	return WriteConfig(file, cfg)
}

func WriteConfig(w io.Writer, cfg *Config) error {
	err := toml.NewEncoder(w).Encode(cfg)
	if err != nil {
		return fmt.Errorf("bot.WriteConfig: %w", err)
	}

	return nil
}

func (cfg *Config) getCredentialsJSON() ([]byte, error) {
	if cfg.CredentialsJSON != "" {
		return []byte(cfg.CredentialsJSON), nil
	}

	if cfg.CredentialsFile != "" {
		b, err := os.ReadFile(cfg.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("bot.Config.getCredentialsJSON: %w", err)
		}
		return b, nil
	}

	return nil, nil
}
