// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func LoadViper(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DXRK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read viper config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal viper config: %w", err)
	}

	return &cfg, nil
}
