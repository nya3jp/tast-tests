// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseBoardConfig(t *testing.T) {
	for name, item := range map[string]struct {
		input  string
		output *BoardConfig
	}{
		"kukui": {
			input: `; Default empty board.ini
`,
			output: &BoardConfig{},
		},
		"vilboz": {
			input: `[ucm]
  ignore_suffix="HD-Audio Generic"
`,
			output: &BoardConfig{
				ucm: boardConfigUCM{
					IgnoreSuffixList: []string{"HD-Audio Generic"},
				},
			},
		},
		// We never had multiple ignore_suffix, but this is the way how
		// cras parses the item
		"multiple ignore_suffix": {
			input: `[ucm]
  ignore_suffix="A,B,C,D"
`,
			output: &BoardConfig{
				ucm: boardConfigUCM{
					IgnoreSuffixList: []string{"A", "B", "C", "D"},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseBoardConfig([]byte(item.input))
			if err != nil {
				t.Fatal("parseBoardConfig() failed")
			}
			if diff := cmp.Diff(item.output, got, cmp.AllowUnexported(BoardConfig{})); diff != "" {
				t.Errorf("-want; +got:\n%s", diff)
			}
		})
	}
}

func TestShouldIgnoreUCMSuffix(t *testing.T) {
	cfg := BoardConfig{
		ucm: boardConfigUCM{
			IgnoreSuffixList: []string{"should-ignore"},
		},
	}

	for cardName, want := range map[string]bool{
		"Loopback":          true,
		"should-ignore":     true,
		"should-not-ignore": false,
	} {
		t.Run(cardName, func(t *testing.T) {
			got := cfg.ShouldIgnoreUCMSuffix(cardName)
			if got != want {
				t.Errorf("cfg.ShouldIgnoreUCMSuffix(%s) = %v != %v", cardName, got, want)
			}
		})
	}
}
