package config

type Config struct {
}

func Load(path string) (*Config, error) {
	return &Config{}, nil
}
