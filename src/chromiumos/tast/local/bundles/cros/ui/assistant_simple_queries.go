// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantSimpleQueries,
		Desc:         "Tests Assistant basic functionality with simple queries",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
	})
}

func AssistantSimpleQueries(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// TODO(b/129896357): Replace the waiting logic once Libassistant has a reliable signal for
	// its readiness to watch for in the signed out mode.
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Libassistant to become ready: ", err)
	}

	testAssistantSimpleMathQuery(ctx, s, tconn)
}

func testAssistantSimpleMathQuery(ctx context.Context, s *testing.State, tconn *chrome.Conn) {
	s.Log("Sending math query to the Assistant")
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "13 + 52 =")
	if err != nil {
		s.Fatal("Failed to get Assistant math query response: ", err)
	}

	s.Log("Verifying the math query result")
	// TODO(meilinw): Remove this logic once we check in the new API.
	fallback := queryStatus.Fallback
	if fallback == "" {
		if queryStatus.QueryResponse.Fallback == "" {
			s.Fatal("No response sent back from Assistant")
		}
		fallback = queryStatus.QueryResponse.Fallback
	}

	// Parses the numeric components from the fallback string.
	re := regexp.MustCompile(`(\d{2})`)
	match := re.FindString(fallback)
	if match == "" {
		s.Fatalf("Fallback string (%v) didn't contain any two-digit numbers", fallback)
	}
	result, err := strconv.Atoi(match)
	if err != nil {
		s.Fatalf("Failed to convert %v to int type: %v", match, err)
	}

	if result != 65 {
		s.Fatalf("Assistant gave the wrong answer: got %d; want 65", result)
	}
}
