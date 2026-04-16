// Package reaction provides a simple way to react to messages in a Discord channel.
package reaction

type Reaction struct {
	Emoji           string    `toml:"emoji"`
	RemovalEmoji    string    `toml:"removal_emoji"`
	Next            *Reaction `toml:"next"`
	ProbabilityNext float32   `toml:"probability_next"`
}
