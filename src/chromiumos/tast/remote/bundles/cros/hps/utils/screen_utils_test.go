// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"testing"
)

func TestPollForDim(t *testing.T) {
	type inputData struct {
		initialBrightness float64
		currentBrightness float64
		checkForDark      bool
	}
	for _, tc := range []struct {
		input  *inputData
		output string
	}{
		// checkForDark: false
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 100,
				checkForDark:      false,
			},
			output: "Auto dim failed. Before human presence: 100.000000, After human presence: 100.000000",
		},
		{
			input: &inputData{
				initialBrightness: 200,
				currentBrightness: 100,
				checkForDark:      false,
			},
			output: "",
		},
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 200,
				checkForDark:      false,
			},
			output: "Auto dim failed. Before human presence: 100.000000, After human presence: 200.000000",
		},
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 0,
				checkForDark:      false,
			},
			output: "Screen went dark unexpectedly",
		},
		// checkForDark: true
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 100,
				checkForDark:      true,
			},
			output: "Auto dim failed. Before human presence: 100.000000, After human presence: 100.000000",
		},
		{
			input: &inputData{
				initialBrightness: 200,
				currentBrightness: 100,
				checkForDark:      true,
			},
			// When looking for screen to turn off we're not interested in brightness
			// changes other than changing to zero.
			output: "Brightness not changed",
		},
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 200,
				checkForDark:      true,
			},
			output: "Auto dim failed. Before human presence: 100.000000, After human presence: 200.000000",
		},
		{
			input: &inputData{
				initialBrightness: 100,
				currentBrightness: 0,
				checkForDark:      true,
			},
			output: "",
		},
	} {
		err := pollForDimHelper(tc.input.initialBrightness, tc.input.currentBrightness, tc.input.checkForDark)
		if err != nil {
			if err.Error() != tc.output {
				t.Errorf("Incorrect Output: expected %q, got %q", tc.output, err.Error())
			}
		} else {
			if tc.output != "" {
				t.Errorf("Incorrect Output: expected %q, got nil", tc.output)
			}
		}
	}
}
