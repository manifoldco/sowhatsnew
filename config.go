package main

import "github.com/hashicorp/hcl"

type Config struct {
	Package string
	Deps    map[string]string
}

func NewConfig(raw string) (*Config, error) {
	c := &Config{}
	err := hcl.Decode(&c, raw)
	return c, err
}
