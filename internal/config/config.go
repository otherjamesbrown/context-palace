package config

type Config struct {
	ClaudeMD string       `yaml:"claude_md"`
	Limits   LimitsConfig `yaml:"limits"`
	Logging  LogConfig    `yaml:"logging"`
}

type LimitsConfig struct {
	ActionLines int `yaml:"action_lines"`
	TriggerRows int `yaml:"trigger_rows"`
}

type LogConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AccessLog string `yaml:"access_log"`
	WritesLog string `yaml:"writes_log"`
}

func DefaultConfig() Config {
	return Config{
		ClaudeMD: "CLAUDE.md",
		Limits: LimitsConfig{
			ActionLines: 30,
			TriggerRows: 20,
		},
		Logging: LogConfig{
			Enabled:   true,
			AccessLog: "logs/access.jsonl",
			WritesLog: "logs/writes.jsonl",
		},
	}
}