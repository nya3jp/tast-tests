// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantBrightnessQueries,
		Desc:         "Tests changing the screen brightness using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// AssistantBrightnessQueries tests that Assistant queries can be used to change screen brightness
func AssistantBrightnessQueries(ctx context.Context, s *testing.State) {
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

	// Set initial brightness
	b0 := "50%"
	if err := assistantBrightnessTest(ctx, tconn, fmt.Sprintf("set brightness to %v", b0), func(actual string) bool {
		return actual == b0
	}); err != nil {
		s.Fatalf("Brightness not set to %v: %v", b0, err)
	}

	// Increase the brightness
	if err := assistantBrightnessTest(ctx, tconn, "turn brightness up", func(actual string) bool {
		return actual > b0
	}); err != nil {
		s.Fatal("Brightness not increased: ", err)
	}

	// Get updated brightness value to verify the following decreasing query
	b1, err := getNodeValue(ctx, tconn, ui.FindParams{Name: "Brightness"})
	if err != nil {
		s.Fatal("Failed to get brightness setting value from ubertray: ", err)
	}

	// Decrease the brightness
	if err := assistantBrightnessTest(ctx, tconn, "turn brightness down", func(actual string) bool {
		return actual < b1
	}); err != nil {
		s.Fatal("Brightness not decreased: ", err)
	}
}

// assistantBrightnessTest is a helper function containing repeated test steps (sends queries and check brightness values via ubertray).
// query contains the Assistant query string.
// predicate is a function containing the expected condition for the actual setting value post-query
func assistantBrightnessTest(ctx context.Context, tconn *chrome.TestConn, query string, predicate func(actual string) bool) error {
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
		actual, err := getNodeValue(ctx, tconn, ui.FindParams{Name: "Brightness"})
		if err != nil {
			return testing.PollBreak(err)
		}
		if !predicate(actual) {
			return errors.Errorf("setting not updated yet, current value is %v", actual)
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
