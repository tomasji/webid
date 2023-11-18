package config

import "github.com/ilyakaznacheev/cleanenv"

// Config of the controllers
type Config struct {
	IngressDomain string `env:"INGRESS_DOMAIN"              env-required:"true"`
	IngressClass  string `env:"INGRESS_CLASS"              env-default:"nginx"`
}

// New creates and initializes configuration
func New() (*Config, error) {
	cfg := &Config{}

	err := cleanenv.ReadEnv(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
