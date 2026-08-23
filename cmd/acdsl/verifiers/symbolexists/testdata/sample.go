// Package sample is a symbolexists fixture: a func, a method, a type, and a
// var to look up. Multiline signature exercises whitespace normalization.
package sample

import "context"

type Config struct {
	Name string
}

var DefaultName = "sample"

func Load(
	ctx context.Context,
	path string,
) (*Config, error) {
	_ = ctx
	_ = path
	return new(Config{}), nil
}

func (c *Config) Validate() error {
	_ = c
	return nil
}
