package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/NotRllyRn/life-prep/internal/core"
	"github.com/NotRllyRn/life-prep/internal/googlemail"
)

type mailboxSync interface {
	Backfill(context.Context, int) ([]googlemail.Message, string, error)
	History(context.Context, string) ([]googlemail.Message, string, error)
}

func syncMessages(ctx context.Context, c mailboxSync, cursor string, force, newOnly bool) ([]googlemail.Message, string, error) {
	if newOnly && (cursor == "" || force) {
		return nil, "", errors.New("NewOnly requires a seeded cursor and forbids backfill; run seed on an empty event database")
	}
	if cursor == "" || force {
		return c.Backfill(ctx, 21)
	}
	ms, next, err := c.History(ctx, cursor)
	var expired *googlemail.HistoryExpiredError
	if errors.As(err, &expired) {
		if newOnly {
			return nil, "", fmt.Errorf("NewOnly history expired; reseed an empty event database: %w", err)
		}
		return c.Backfill(ctx, -1)
	}
	return ms, next, err
}

// Fetch all baselines before writing any, so partial API failure changes nothing.
func seed(ctx context.Context, s *core.Store, accounts []Account, profile func(context.Context, Account) (string, error)) error {
	events, err := s.List()
	if err != nil {
		return err
	}
	if len(events) != 0 {
		return errors.New("seed refuses existing events; use a fresh database to avoid old pending dispatch")
	}
	baselines := make(map[string]string, len(accounts))
	for _, a := range accounts {
		cursor, err := profile(ctx, a)
		if err != nil {
			return fmt.Errorf("seed account %q: %w", a.ID, err)
		}
		baselines[a.ID] = cursor
	}
	return s.Seed(baselines)
}
