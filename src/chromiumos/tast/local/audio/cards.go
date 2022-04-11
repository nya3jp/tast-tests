// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

// Card is a card listed in /proc/asound/cards as:
// ## [ID             ]: Driver - ShortName
//                       LongName
//
// snd_iprintf(buffer, "%2i [%-15s]: %s - %s\n",
// 	idx,
// 	card->id,
// 	card->driver,
// 	card->shortname);
// snd_iprintf(buffer, "                      %s\n",
// 	card->longname);
type Card struct {
	ID        string
	Driver    string
	ShortName string
	LongName  string
}

// cardRegexp parses the string from printf "%2i [%-15s]: %s - %s\n"
// See doc for Card.
var cardRegexp = regexp.MustCompile(`[ \d]{2} \[(.{15})\]: (\S+) - (.+)
\s+(.+)
`)

// parseSoundCards parses Cards from s in /proc/asound/cards's format
func parseSoundCards(s string) ([]Card, error) {
	if s == "--- no soundcards ---\n" {
		return nil, nil
	}

	if s == "" {
		return nil, errors.New("unexpected empty file")
	}

	lines := strings.Count(s, "\n")
	if lines%2 != 0 {
		return nil, errors.Errorf("expected even number of lines, got %d", lines)
	}

	var cards []Card
	for _, m := range cardRegexp.FindAllStringSubmatch(s, -1) {
		cards = append(cards, Card{
			ID:        strings.TrimSpace(m[1]),
			Driver:    m[2],
			ShortName: m[3],
			LongName:  m[4],
		})
	}

	if expectedCards := lines / 2; expectedCards != len(cards) {
		return nil, errors.Errorf(
			"expected to find %d cards from %d lines, but found %d cards",
			expectedCards, lines, len(cards))
	}

	return cards, nil
}

// GetSoundCards returns Cards from /proc/asound/cards
func GetSoundCards() ([]Card, error) {
	b, err := os.ReadFile("/proc/asound/cards")
	if err != nil {
		return nil, errors.Errorf("cannot parse /proc/asound/cards: %s", err)
	}
	return parseSoundCards(string(b))
}
