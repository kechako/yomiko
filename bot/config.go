package bot

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

type Replacement struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

type Config struct {
	Token           string         `toml:"token"`
	CredentialsJSON string         `toml:"credentials_json"`
	CredentialsFile string         `toml:"credentials_file"`
	DatabasePath    string         `toml:"database_path"`
	Replacements    []*Replacement `toml:"replacements"`
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
