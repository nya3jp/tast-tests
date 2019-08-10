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
	})
}

func AssistantSimpleQueries(ctx context.Context, s *testing.State) {
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

	// TODO(b/129896357): Replace the waiting logic once Libassistant has a reliable signal for
	// its readiness to watch for in the signed out mode.
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Error("Failed to wait for Libassistant to become ready: ", err)
	}

	testAssistantSimplemathQuery(ctx, tconn, s)
}

func testAssistantSimplemathQuery(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	s.Log("Sending math query to the Assistant")
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "13 + 52 =")
	if err != nil {
		s.Error("Failed to get Assistant time response: ", err)
		return
	}

	s.Log("Verifying the math query result")
	// TODO(meilinw): Remove this logic once we check in the new API.
	fallback := queryStatus.Fallback
	if fallback == "" {
		if queryStatus.QueryResponse.Fallback == "" {
			s.Error("No response sent back from Assistant")
			return
		}
		fallback = queryStatus.QueryResponse.Fallback
	}

	// Parses the numeric components from the fallback string.
	re := regexp.MustCompile(`(\d{2})`)
	match := re.FindString(fallback)
	if match == "" {
		s.Errorf("Fallback string (%v) doesn't contain any numeric components", fallback)
		return
	}
	result, err := strconv.Atoi(match)
	if err != nil {
		s.Errorf("Failed to convert %v to int type: %v", match, err)
		return
	}

	if result != 65 {
		s.Errorf("Assistant gives the wrong answer: %d", result)
	}
}
