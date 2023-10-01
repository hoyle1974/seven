package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog/log"
)

var cache, _ = lru.New[string, Entry](1024)

type Entry struct {
	uuid     uuid.UUID
	address  string
	lastSeen time.Time
}

func (e Entry) ToEntryJson() EntryForm {
	return EntryForm{
		Uuid:    e.uuid.String(),
		Address: e.address,
	}
}

func genGoodRandom(max int, bad map[int]bool) int {
	n := -1
	maxTries := 5
	for maxTries > 0 {
		p := rand.Intn(max)
		if !bad[p] {
			n = p
			break
		}
		maxTries--
	}
	return n
}

func pickSome(values []Entry, amount int) []EntryForm {
	picked := make([]EntryForm, 0)
	bad := make(map[int]bool, 0)

	if len(values) == 0 {
		return picked
	}

	for amount > 0 {
		idx := genGoodRandom(len(values), bad)
		if idx == -1 {
			break
		}
		bad[idx] = true
		picked = append(picked, values[idx].ToEntryJson())
		amount--
	}

	return picked
}

func registerJSON(json EntryForm) ([]EntryForm, error) {
	entries := []EntryForm{}

	// Extract and validate uuid
	uuid, err := uuid.Parse(json.Uuid)
	if err != nil {
		return entries, fmt.Errorf("Error converting uuid string ot actual uuid")
	}
	if len(json.Address) < 1 {
		return entries, fmt.Errorf("Address was empty")
	}

	entries = pickSome(cache.Values(), 16)

	entry := Entry{
		uuid:     uuid,
		address:  json.Address,
		lastSeen: time.Now(),
	}

	// Store this uuid and it's address
	log.Debug().Str("uuid", json.Uuid).Msg("Registering client")
	cache.Add(json.Uuid, entry)

	return entries, nil
}
