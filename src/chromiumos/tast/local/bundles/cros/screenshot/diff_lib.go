// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot contains code to test the screenshot library.
package screenshot

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiffLib,
		Desc:         "Test to confirm that the diffing library works as intended",
		Contacts:     []string{"msta@google.com", "chrome-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{screenshot.GoldServiceAccountKeyVar},
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
	if diff := cmp.Diff(expected, actual); diff != "" {
		// The linter complains about the newline on the next line.
		s.Fatalf("Unexpected set of failing diffs (-want +got):\n%s", diff) // NOLINT
	}
}

func DiffLib(ctx context.Context, s *testing.State) {
	d, _, err := screenshot.NewDiffer(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize differ: ", err)
	}

	launcher := nodewith.ClassName("ash/HomeButton")
	searchBox := nodewith.ClassName("SearchBoxView").Role(role.Group)

	if err := d.Diff("nomatches", nodewith.ClassName("MissingClassName")); err == nil {
		s.Fatal("Expected no matches")
	}
	if err := d.Diff("multiplematches", nodewith.ClassName("FrameCaptionButton")); err == nil {
		s.Fatal("Expected multiple matches")
	}

	if err := d.Diff("repeat", launcher); err != nil {
		s.Fatal("Failed to send diff: ", err)
	}
	if err := d.Diff("repeat", launcher); err == nil {
		s.Fatal("Expected sending the same diff twice to fail")
	}

	expectFailedDiffs(parseFailedDiffs(d.GetFailedDiffs()), s, "repeat")

	diffs := parseFailedDiffs(screenshot.DiffPerConfig(ctx, s, []screenshot.Config{
		{Region: "us"},
		{Region: "au"},
		{Region: "jp"},
	}, func(d screenshot.Differ, cr *chrome.Chrome) {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create tconn: ", err)
		}
		ui2 := uiauto.New(tconn).WithTimeout(time.Second * 3)
		if err := uiauto.Run(ctx,
			d.DiffAction("launcher", launcher),
			ui2.LeftClick(launcher),
			ui2.WaitUntilExists(searchBox),
			func(ctx context.Context) error { return ui.WaitForLocationChangeCompleted(ctx, tconn) },
			d.DiffAction("searchbox", searchBox),
		); err != nil {
			s.Fatal("Failed to screenshot searchbox: ", err)
		}
	}))

	// Launcher-au expect be approved so that it doesn't fail.
	// If this test is failing, there may have been some ui changes which means you may need to re-approve the launcher.au diff.
	expectFailedDiffs(diffs, s, "searchbox", "searchbox.au", "searchbox.jp", "launcher", "launcher.jp")

	// The search box should look the same in australia and the US, but different in japan.
	if diffs["searchbox"] != diffs["searchbox.au"] {
		s.Fatal("searchbox != searchbox.au")
	}
	if diffs["searchbox"] == diffs["searchbox.jp"] {
		s.Fatal("searchbox == searchbox.jp")
	}

	// The launcher should look different to the search box, but should be the same in both the US and Japan
	if diffs["searchbox"] == diffs["launcher"] {
		s.Fatal("searchbox == launcher")
	}
	if diffs["launcher"] != diffs["launcher.jp"] {
		s.Fatal("launcher != launcher.jp")
	}
}
