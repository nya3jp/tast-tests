// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiffLib,
		Desc:         "Test to confirm that the diffing library works as intended",
		Contacts:     []string{"msta@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"goldServiceAccountKey"},
	})
}

// parseFailedDiffs returns a map from test name to the hash of the image.
func parseFailedDiffs(diffs error) map[string]string {
	results := map[string]string{}
	resultRe := regexp.MustCompile(`Untriaged or negative image: .*\?test=screenshot\.DiffLib\.([a-zA-Z0-9\-.]*)&digest=([a-z0-9]+)`)
	errors := strings.Split(diffs.Error(), "\n\n")[1:]
	for _, err := range errors {
		result := resultRe.FindSubmatch([]byte(err))
		results[string(result[1])] = string(result[2])
	}
	return results
}

func expectFailedDiffs(diffs map[string]string, s *testing.State, expected ...string) {
	var actual []string
	for key := range diffs {
		actual = append(actual, key)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) {
		s.Fatalf("Got %v, but expected %v", actual, expected)
	}
}

func DiffLib(ctx context.Context, s *testing.State) {
	expectSuccess := func(msg string, err error) {
		if err != nil {
			s.Fatal(msg, err)
		}
	}
	expectFail := func(msg string, err error) {
		if err == nil {
			s.Fatal(msg)
		}
	}
	expectEq := func(lhs, rhs string) {
		if lhs != rhs {
			s.Fatalf("%s != %s", lhs, rhs)
		}
	}
	expectNe := func(lhs, rhs string) {
		if lhs == rhs {
			s.Fatalf("%s == %s", lhs, rhs)
		}
	}

	d, _, err := screenshot.NewDiffer(ctx, s)
	expectSuccess("Failed to initialize differ: ", err)

	launcherParams := ui.FindParams{ClassName: "ash/HomeButton"}

	expectFail("Expected no matches", d.Diff("nomatches", ui.FindParams{ClassName: "MissingClassName"}))
	expectFail("Expected multiple matches", d.Diff("multiplematches", ui.FindParams{ClassName: "FrameCaptionButton"}))

	expectSuccess("Failed to send diff: ", d.Diff("repeat", launcherParams))
	expectFail("Expected sending the same diff twice to fail", d.Diff("repeat", launcherParams))

	expectFailedDiffs(parseFailedDiffs(d.GetFailedDiffs()), s, "repeat")

	diffs := parseFailedDiffs(screenshot.DiffPerConfig(ctx, s, []screenshot.Config{
		{Region: "us"},
		{Region: "au"},
		{Region: "jp"},
	}, func(d screenshot.Differ, cr *chrome.Chrome) {
		expectSuccess("Taking screenshot of launcher failed: ", d.Diff("launcher", launcherParams))

		tconn, err := cr.TestAPIConn(ctx)
		expectSuccess("Getting TestApiConn failed", err)
		expectSuccess("Attempted to open the shelf: ", uig.Do(ctx, tconn, uig.FindWithTimeout(launcherParams, 3*time.Second).LeftClick().WaitForLocationChangeCompleted()))
		searchBox, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "SearchBoxView"}, 3*time.Second)
		expectSuccess("Attempted to find node", err)
		expectSuccess("Taking screenshot of search box failed: ", d.DiffNode("searchbox", searchBox))
	}))

	// Launcher-au expect be approved so that it doesn't fail.
	// If this test is failing, there may have been some ui changes which means you may need to re-approve the launcher.au diff.
	expectFailedDiffs(diffs, s, "searchbox", "searchbox.au", "searchbox.jp", "launcher", "launcher.jp")

	// The search box should look the same in australia and the US, but different in japan.
	expectEq(diffs["searchbox"], diffs["searchbox.au"])
	expectNe(diffs["searchbox"], diffs["searchbox.jp"])

	// The launcher should look different to the search box, but should be the same in both the US and Japan
	expectNe(diffs["searchbox"], diffs["launcher"])
	expectEq(diffs["launcher"], diffs["launcher.jp"])
}
