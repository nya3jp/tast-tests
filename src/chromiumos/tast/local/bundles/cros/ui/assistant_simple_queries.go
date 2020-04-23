// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantSimpleQueries,
		Desc:         "Tests Assistant basic functionality with simple queries",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
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

func testAssistantSimpleMathQuery(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
	s.Log("Sending math query to the Assistant")
	// As the query result will be parsed from the HTML string, a special number is needed to
	// be distinguished from other numeric components contained in the HTML.
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "1234567 + 7654321 =")
	if err != nil {
		s.Fatal("Failed to get Assistant math query response: ", err)
	}

	s.Log("Verifying the math query result")
	html := queryStatus.QueryResponse.HTML
	if html == "" {
		s.Fatal("No HTML response sent back from Assistant")
	}

	// The html string should contain the answer of the math query.
	if !strings.Contains(html, "8 888 888") {
		s.Fatal("HTML response doesn't contain the correct answer of the math query: ", html)
	}
}
