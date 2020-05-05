// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
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

	// Set initial brightness and verify Ubertray brightness slider updated
	b0 := 50
	s.Log("Setting initial brightness")
	if err := brightnessUICheck(ctx, tconn, fmt.Sprintf("set brightness to %v%%", b0), func(actual int) bool {
		return actual == b0
	}); err != nil {
		s.Fatalf("Ubertray brightness slider not set to %v: %v", b0, err)
	}

	// Get the actual brightness using backlight_tool util before running next query
	previous, err := getBrightness(ctx)
	if err != nil {
		s.Fatal("Failed to get brightness value: ", err)
	}
	s.Log("Actual brightness before increasing query: ", previous)

	// Increase the brightness and check that the Ubertray brightness slider increased
	s.Log("Increasing the brightness")
	if err := brightnessUICheck(ctx, tconn, "turn brightness up", func(actual int) bool {
		return actual > b0
	}); err != nil {
		s.Fatal("Ubertray brightness slider not increased: ", err)
	}

	// Check that the actual brightness level has increased using the backlight_tool util
	s.Log("Checking brightness using backlight_tool")
	if err := testing.Poll(ctx, func(context.Context) error {
		current, err := getBrightness(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if current <= previous {
			return errors.Errorf("actual br	ightness did not increase yet; before query: %v, current: %v", previous, current)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Brightness setting not increased by query: ", err)
	}

	// Get actual brightness before next query
	previous, err = getBrightness(ctx)
	if err != nil {
		s.Fatal("Failed to get brightness value: ", err)
	}
	s.Log("Actual brightness before decreasing query: ", previous)

	// Get updated Ubertray brightness slider value to verify the following decreasing query
	b, err := getNodeValue(ctx, tconn, ui.FindParams{Name: "Brightness"})
	if err != nil {
		s.Fatal("Failed to get Ubertray brightness slider value: ", err)
	}

	// The UI node value is a string like "50%". Convert to int for comparison
	b1, err := strconv.Atoi(b[:len(b)-1])
	if err != nil {
		s.Fatal("Failed to convert brightness percentage string to int: ", err)
	}

	// Decrease the brightness
	s.Log("Decreasing the brightness")
	if err := brightnessUICheck(ctx, tconn, "turn brightness down", func(actual int) bool {
		return actual < b1
	}); err != nil {
		s.Fatal("Ubertray brightness slider not decreased: ", err)
	}

	s.Log("Checking brightness using backlight_tool")
	if err := testing.Poll(ctx, func(context.Context) error {
		current, err := getBrightness(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if current >= previous {
			return errors.Errorf("actual brightness did not decrease yet; before query: %v, current: %v", previous, current)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Brightness not decreased by query: ", err)
	}
}

// brightnessUICheck is a helper function containing repeated test steps (sends queries and check brightness values via ubertray).
// query contains the Assistant query string.
// predicate is a function containing the expected condition for the actual setting value post-query
func brightnessUICheck(ctx context.Context, tconn *chrome.TestConn, query string, predicate func(actual int) bool) error {
	// Queries sometimes don't go through when running several queries in the test, so poll to ensure the queries run.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := assistant.SendTextQuery(ctx, tconn, query)
		return err
	}, nil); err != nil {
		return errors.Wrap(err, "failed to get Assistant query response")
	}

	// Check values in the ubertray
	if err := openUberTray(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open Uber Tray")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v, err := getNodeValue(ctx, tconn, ui.FindParams{Name: "Brightness"})
		if err != nil {
			return testing.PollBreak(err)
		}
		actual, err := strconv.Atoi(v[:len(v)-1])
		if !predicate(actual) {
			return errors.Errorf("brightness slider not updated as expected yet; current value: %v", actual)
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "brightness not set successfully by the assistant query")
	}

	return nil
}

// openUberTray clicks the status area to open the ubertray
func openUberTray(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return err
	}
	defer statusArea.Release(ctx)

	return statusArea.LeftClick(ctx)
}

// getNodeValue gets the Value field of an automation node
func getNodeValue(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (string, error) {
	n, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return "", errors.Wrap(err, "failed to find node")
	}
	defer n.Release(ctx)

	return n.Value, nil
}

func getBrightness(ctx context.Context) (float64, error) {
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
