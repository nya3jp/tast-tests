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

var cardRegexp = regexp.MustCompile(`[ \d]{2} \[(.{15})\]: (\S+) - (.+)
\s+(.+)
`)

func parseSoundCards(s string) (cards []Card, err error) {
	if s == "--- no soundcards ---\n" {
		return
	}

	matches := cardRegexp.FindAllStringSubmatch(s, -1)
	if cards, lines := len(matches), strings.Count(s, "\n"); lines != cards*2 {
		return nil, errors.Errorf(
			"found %d cards from %d lines, should find %g cards instead",
			cards, lines, float64(lines)/2)
	}

	for _, m := range matches {
		cards = append(cards, Card{
			strings.TrimSpace(m[1]),
			m[2],
			m[3],
			m[4],
		})
	}
	return
}

// GetSoundCards returns Cards from /proc/asound/cards
func GetSoundCards() (cards []Card, err error) {
	b, err := os.ReadFile("/proc/asound/cards")
	if err != nil {
		return
	}
	return parseSoundCards(string(b))
}
