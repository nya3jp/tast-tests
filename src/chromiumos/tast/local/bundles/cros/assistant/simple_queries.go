// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SimpleQueries,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests Assistant basic functionality with simple queries",
		Contacts: []string{
			"yawano@google.com",
			"updowndota@chromium.org",
			"xiaohuic@chromium.org",
			"assistive-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Params: []testing.Param{{
			Fixture: "assistant",
		}, {
			Name:              "with_android_p",
			Fixture:           "assistantWithArc",
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "with_android_vm",
			Fixture:           "assistantWithArc",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SimpleQueries(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	testAssistantSimpleMathQuery(ctx, s, tconn)
}

func testAssistantSimpleMathQuery(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
	s.Log("Sending math query to the Assistant")
	// As the query result will be parsed from the HTML string, a special number is needed to
	// be distinguished from other numeric components contained in the HTML.
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "-214.5 - 785.4 =")
	if err != nil {
		s.Fatal("Failed to get Assistant math query response: ", err)
	}

	s.Log("Verifying the math query result")
	html := queryStatus.QueryResponse.HTML
	if html == "" {
		s.Fatal("No HTML response sent back from Assistant")
	}

	// The HTML string should contain the answer of the math query.
	if !strings.Contains(html, "-999.9") {
		// Writes the HTML response to logName file for debugging if no matching results found.
		const logName = "math_query_html_response.txt"
		s.Log("No matching results found. Try to log the HTML response to ", logName)
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), logName), []byte(html), 0644); err != nil {
			s.Logf("Failed to log response to %s: %v", logName, err)
		}
		s.Fatal("HTML response doesn't contain the answer of the math query")
	}
}
