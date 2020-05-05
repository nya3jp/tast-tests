// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrightnessQueries,
		Desc:         "Tests changing the screen brightness using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org", "meilinw@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// BrightnessQueries tests that Assistant queries can be used to change screen brightness
func BrightnessQueries(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Libassistant to become ready: ", err)
	}

	// Set initial brightness with the Assistant. Initial value is not confirmed with backlight_tool,
	// since the UI brightness value does not map to the backlight_tool value in a straightforward way.
	// We are not using backlight_tool to set the initial brightness - the UI does not register this change,
	// and the Assistant queries will change the brightness relative to what the setting was before it was
	// changed by backlight_tool.
	s.Log("Setting initial brightness")
	// Set brightness to a specific value
	b0, err := brightnessCheck(ctx, tconn, "set brightness to 50%", func(actual float64) bool {
		return true
	})
	if err != nil {
		s.Fatal("Brightness (UI value) not initialized to 50%: ", err)
	}
	s.Log("Initial backlight_tool brightness value b0: ", b0)

	// Increase the brightness and check that the Ubertray brightness slider increased
	s.Log("Increasing the brightness")
	b1, err := brightnessCheck(ctx, tconn, "turn brightness up", func(actual float64) bool {
		return actual > b0
	})
	if err != nil {
		s.Fatal("Brightness not increased: ", err)
	}
	s.Log("b1: ", b1)

	// Decrease the brightness
	s.Log("Decreasing the brightness")
	b2, err := brightnessCheck(ctx, tconn, "turn brightness down", func(actual float64) bool {
		return actual < b1
	})
	if err != nil {
		s.Fatal("Brightness not decreased: ", err)
	}
	s.Log("b2: ", b2)
}

// brightnessCheck sends assistant brightness queries and checks their result.
// Returns the backlight_tool brightness percentage after the query.
// query contains the Assistant query string.
// predicate is a function containing the expected condition for the actual setting value post-query
func brightnessCheck(ctx context.Context, tconn *chrome.TestConn, query string, predicate func(actual float64) bool) (float64, error) {
	if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
		return 0, errors.Wrap(err, "failed to get Assistant query response")
	}

	// Check brightness value with backlight_tool
	if err := testing.Poll(ctx, func(context.Context) error {
		current, err := brightness(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if !predicate(current) {
			return errors.Errorf("brightness did not change yet; brightness: %v", current)
		}
		return nil
	}, nil); err != nil {
		return 0, errors.Wrap(err, "brightness setting not increased by query")
	}

	return brightness(ctx)
}

func brightness(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to run backlight_tool command")
	}
	b, err := strconv.ParseFloat(strings.TrimSuffix(string(out), "\n"), 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert backlight_tool output to numeric value")
	}
	return b, nil
}
