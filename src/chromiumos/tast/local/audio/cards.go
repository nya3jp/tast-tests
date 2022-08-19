// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// Card is a card listed in /proc/asound/cards as:
//
//	## [ID             ]: Driver - ShortName
//	                      LongName
//
// It was formatted with the below, from sound/core/init.c:
//
//	snd_iprintf(buffer, "%2i [%-15s]: %s - %s\n",
//			idx,
//			card->id,
//			card->driver,
//			card->shortname);
//	snd_iprintf(buffer, "                      %s\n",
//			card->longname);
type Card struct {
	Index     int
	ID        string
	Driver    string
	ShortName string
	LongName  string
}

// cardRegexp parses the string from printf "%2i [%-15s]: %s - %s\n"
// See doc for Card.
var cardRegexp = regexp.MustCompile(`([ \d]){2} \[(.{15})\]: (\S+) - (.+)
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
		index, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, errors.Errorf("cannot parse %q as an integer", m[1])
		}
		cards = append(cards, Card{
			Index:     index,
			ID:        strings.TrimSpace(m[2]),
			Driver:    m[3],
			ShortName: m[4],
			LongName:  m[5],
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

// IsExternal tells whether the sound card is an external card.
func (c *Card) IsExternal() (bool, error) {
	bus, err := filepath.EvalSymlinks(fmt.Sprintf("/sys/class/sound/card%d/device/subsystem", c.Index))
	if err != nil {
		return false, err
	}
	switch bus {
	case "/sys/bus/platform", "/sys/bus/pci":
		return false, nil
	case "/sys/bus/usb":
		return true, nil
	default:
		return false, errors.Errorf("unknown bus: %q", bus)
	}
}
