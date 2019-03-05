// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantTextQueries,
		Desc:         "Tests the functionalities of Assistant based on text query",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func AssistantTextQueries(ctx context.Context, s *testing.State) {
	const (
		timeQuery = "what time is it in UTC?"
	)

	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=ChromeOSAssistant"))
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// Tests time query and verifies the returned time response.
	response, err := assistant.SendTextQuery(ctx, tconn, timeQuery)
	if err != nil {
		s.Fatal("Failed to get Assistant response: ", err)
	}
	htmlFallback, ok := response["htmlFallback"]
	if !ok {
		s.Fatal("No htmlfallback available in the response")
	}
	verifyTimeResponse(s, htmlFallback)
}

// verifyTimeResponse compares the assistant time response with the UTC time.
func verifyTimeResponse(s *testing.State, htmlFallback string) {
	assistantTime := parseTimeResponse(s, htmlFallback)
	expectedTime := time.Now().UTC()

	if isTimeMatch(assistantTime, expectedTime) {
		return
	}

	// Time string may not be identical due to response latency or clock drift
	// Further comparison is needed when this happened to make sure the time result
	// is within maximum tolerance.
	tolerance := time.Minute
	expectedTimeDriftForward := expectedTime.Add(tolerance)
	expectedTimeDriftBackward := expectedTime.Add(-tolerance)
	if !(isTimeMatch(assistantTime, expectedTimeDriftForward) ||
		isTimeMatch(assistantTime, expectedTimeDriftBackward)) {
		s.Fatal("Returned time doesn't match the current UTC time")
	}
}

// parseTimeResponse extracts and formats the numeric components in the htmlFallback string
// to the HH:MM representation in 12 hour AM/PM.
func parseTimeResponse(s *testing.State, htmlFallback string) string {
	re := regexp.MustCompile(`(\d{1,2}:\d{1,2})|(\d{1,2})`)
	assistantTime := re.FindString(htmlFallback)
	if len(assistantTime) == 0 {
		s.Fatal("Returned html fallback doesn't contain any well-formatted time results")
	}

	// Parses hours and minutes by splitting a colon. There are two possible time formats in the
	// htmlFallback string: HH/H:MM (e.g. "It's 5:20 PM.") or HH/H ("It's exactly 12 o'clock.").
	var hrs, min string
	timeCompo := strings.Split(assistantTime, ":")
	hrs = timeCompo[0]
	if len(timeCompo) > 1 {
		min = timeCompo[1]
	}
	return fmt.Sprintf("%02s:%02s", hrs, min)
}

// isTimeMatch compares the string format (HH:MM in 12 hour AM/PM) of two time results.
func isTimeMatch(assistantTime string, expectedTime time.Time) bool {
	return assistantTime == expectedTime.Format("03:04")
}
