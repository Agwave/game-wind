// Package config 加载配置文件：config.yaml（模板）+ config.local.yaml（本地覆盖）。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Region 一个地区的判定阈值。
type Region struct {
	GrossingNewTop  int `yaml:"grossing_new_top"`
	GrossingRise    int `yaml:"grossing_rise"`
	GrossingTopBand int `yaml:"grossing_top_band"`
	FreeNewTop      int `yaml:"free_new_top"`
	FreeRise        int `yaml:"free_rise"`
}

// Notify 通知配置。
type Notify struct {
	WebhookURL      string `yaml:"webhook_url"`
	NotifyWhenQuiet bool   `yaml:"notify_when_quiet"`
}

// Config 总配置。
type Config struct {
	Limit   int               `yaml:"limit"`
	Regions map[string]Region `yaml:"regions"`
	Notify  Notify            `yaml:"notify"`
}

// DefaultRegions 默认四地区。
var DefaultRegions = []string{"cn", "us", "jp", "kr"}

// Load 按顺序加载多个 YAML 配置文件，后者覆盖前者的同名键，并填充默认值。
func Load(paths ...string) (*Config, error) {
	cfg := &Config{Limit: 100, Regions: make(map[string]Region)}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // 本地配置文件不存在时忽略
			}
			return nil, fmt.Errorf("读取配置 %s: %w", p, err)
		}
		if err := yaml.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("解析配置 %s: %w", p, err)
		}
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 100
	}
	if len(cfg.Regions) == 0 {
		for _, cc := range DefaultRegions {
			cfg.Regions[cc] = defaultRegion()
		}
	}
	for cc := range cfg.Regions {
		cfg.Regions[cc] = fillRegion(cfg.Regions[cc])
	}
	return cfg, nil
}

func defaultRegion() Region {
	return Region{GrossingNewTop: 50, GrossingRise: 30, GrossingTopBand: 10, FreeNewTop: 20, FreeRise: 30}
}

func fillRegion(r Region) Region {
	d := defaultRegion()
	if r.GrossingNewTop == 0 {
		r.GrossingNewTop = d.GrossingNewTop
	}
	if r.GrossingRise == 0 {
		r.GrossingRise = d.GrossingRise
	}
	if r.GrossingTopBand == 0 {
		r.GrossingTopBand = d.GrossingTopBand
	}
	if r.FreeNewTop == 0 {
		r.FreeNewTop = d.FreeNewTop
	}
	if r.FreeRise == 0 {
		r.FreeRise = d.FreeRise
	}
	return r
}
