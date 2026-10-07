package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
)

func decodeConfig(b []byte) (Config, error) {
	var c Config
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("config must contain exactly one JSON object")
	}
	return c, validateConfig(c)
}

func validateConfig(c Config) error {
	if c.Database == "" || c.Artifacts == "" {
		return errors.New("database and artifacts required")
	}
	ids, emails := map[string]bool{}, map[string]bool{}
	for _, a := range c.Accounts {
		if strings.TrimSpace(a.ID) == "" || ids[a.ID] {
			return errors.New("account IDs must be nonempty and unique")
		}
		ids[a.ID] = true
		email := strings.ToLower(strings.TrimSpace(a.Email))
		if email == "" {
			if c.Enabled {
				return errors.New("enabled accounts require emails")
			}
			continue
		}
		if emails[email] {
			return errors.New("account emails must be unique")
		}
		emails[email] = true
	}
	if c.Enabled && len(c.Accounts) != 2 {
		return errors.New("configure exactly two accounts")
	}
	if c.Hermes.Enabled && (c.Hermes.URL == "" || c.Hermes.SecretEnv == "") {
		return errors.New("Hermes URL and secret environment name required")
	}
	if c.Hermes.URL != "" {
		u, err := url.Parse(c.Hermes.URL)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("invalid Hermes URL")
		}
		ip := net.ParseIP(u.Hostname())
		loopback := strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
		if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
			return errors.New("Hermes URL requires HTTPS or HTTP loopback; never send authentication over remote cleartext")
		}
	}
	return nil
}
