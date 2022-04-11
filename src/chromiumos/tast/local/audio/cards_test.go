// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseSoundCards(t *testing.T) {
	table := map[string]struct {
		input  string
		output []Card
	}{
		"one card": {
			// helios
			input: ` 0 [sofcmlrt1011rt5]: sof-cml_rt1011_ - sof-cml_rt1011_rt5682
                      Google-Helios-rev1-Helios
`,
			output: []Card{
				{"sofcmlrt1011rt5", "sof-cml_rt1011_", "sof-cml_rt1011_rt5682", "Google-Helios-rev1-Helios"},
			},
		},
		"two cards": {
			// grunt
			input: ` 0 [acpd7219m98357 ]: acpd7219m98357 - acpd7219m98357
                      Google-Grunt-rev6
 1 [HDMI           ]: HDA-Intel - HDA ATI HDMI
                      HDA ATI HDMI at 0xf4d80000 irq 43
`,
			output: []Card{
				{"acpd7219m98357", "acpd7219m98357", "acpd7219m98357", "Google-Grunt-rev6"},
				{"HDMI", "HDA-Intel", "HDA ATI HDMI", "HDA ATI HDMI at 0xf4d80000 irq 43"},
			},
		},
		"no cards": {
			input:  "--- no soundcards ---\n",
			output: nil,
		},
	}

	for name, item := range table {
		t.Run(name, func(t *testing.T) {
			cards, err := parseSoundCards(item.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(item.output, cards); diff != "" {
				t.Errorf("parsed cards differ (-want; +got)\n%s", diff)
			}
		})
	}
}

func TestParseSoundCardsError(t *testing.T) {
	_, err := parseSoundCards(` 0 [sofcmlrt1011rt5]: sof-cml_rt1011_ - sof-cml_rt1011_rt5682
	Google-Helios-rev1-Helios
	--this is an extra line--
`)
	if err == nil {
		t.Fatal("parseSoundCards should return an error")
	}
	if err.Error() != "found 1 cards from 3 lines, should find 1.5 cards instead" {
		t.Fatalf("unexpected error message %q", err)
	}
}
