package cfg

import (
	"time"

	"github.com/spf13/viper"
)

type RPC struct {
	URL      string `mapstructure:"url"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type Youtube struct {
	Enabled       bool   `mapstructure:"youtube_enabled"`
	Proxy         string `mapstructure:"proxy"`
	DownloadPath  string `mapstructure:"download_path"`
	YtDlpLocation string `mapstructure:"yt_dlp_location"`
}

type Config struct {
	BotToken        string            `mapstructure:"bot_token"`
	RPC             RPC               `mapstructure:"transmission_rpc"`
	PollIntervalSec int               `mapstructure:"poll_interval_sec"`
	ChatWhitelist   []int64           `mapstructure:"chat_whitelist"`
	Categories      map[string]string `mapstructure:"categories"`
	Youtube         Youtube           `mapstructure:"youtube"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("json")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	if c.PollIntervalSec <= 0 {
		c.PollIntervalSec = 30
	}
	return &c, nil
}

func (c *Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSec) * time.Second
}
