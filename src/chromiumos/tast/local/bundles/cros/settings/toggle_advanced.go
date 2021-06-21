// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ToggleAdvanced,
		Desc: "Checks that the Advanced section of Settings can be expanded and collapsed",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// ToggleAdvanced tests that we can toggle the Advanced Settings section.
func ToggleAdvanced(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}
	if err := settings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed waiting for Settings to load: ", err)
	}

	// Count the initial number of subsections before expanding the Advanced section.
	n, err := settings.NodesInfo(ctx, nodewith.Role(role.Heading))
	if err != nil {
		s.Fatal("Failed to find subsection headings: ", err)
	}
	initialCount := len(n)

	// Expand the Advanced section and count the number of subsections. It should be greater than when it was collapsed.
	advBtn := nodewith.Role(role.Button).Ancestor(nodewith.Role(role.Heading).Name("Advanced"))
	if err := uiauto.Combine("Scroll the Advanced button into view by focusing it, click it, and wait for it to be expanded",
		settings.FocusAndWait(advBtn),
		settings.LeftClick(advBtn),
		settings.WaitUntilExists(advBtn.State(state.Expanded, true)),
	)(ctx); err != nil {
		s.Fatal("Failed to expand the Advanced section: ", err)
	}
	n, err = settings.NodesInfo(ctx, nodewith.Role(role.Heading))
	if err != nil {
		s.Fatal("Failed to find subsection headings: ", err)
	}
	if !(len(n) > initialCount) {
		s.Fatalf("Number of subsections did not increase after expanding the Advanced section. Before: %v, After: %v", initialCount, len(n))
	}

	// Collapse the button and re-count the number of subsections. It should match the initial value when the Advanced section was collapsed.
	if err := uiauto.Combine("Click the advanced button and wait for it to be collapsed",
		settings.LeftClick(advBtn),
		settings.WaitUntilExists(advBtn.State(state.Collapsed, true)),
	)(ctx); err != nil {
		s.Fatal("Failed to collapse the Advanced section: ", err)
	}
	n, err = settings.NodesInfo(ctx, nodewith.Role(role.Heading))
	if err != nil {
		s.Fatal("Failed to find subsection headings: ", err)
	}
	if len(n) != initialCount {
		s.Fatalf("Number of subsections after collapsing the Advanced section did not match the initial value. Expected: %v, Actual: %v", initialCount, len(n))
	}
}
