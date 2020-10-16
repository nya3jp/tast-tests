// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

// testFunc contains the contents of the test itself.
// type testFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn)

func init() {
	testing.AddTest(&testing.Test{
		Func: EditBookmarksEnabled,
		Desc: "Behavior of EditBookmarksEnabled policy",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// EditBookmarksEnabled tests check the enabled/disabled/unset EditBookmarksEnabaled policy
func EditBookmarksEnabled(ctx context.Context, s *testing.State) {
	chrome := s.PreValue().(*pre.PreData).Chrome
	fakeDms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		name          string                       // name is the subtest name.
		value         *policy.EditBookmarksEnabled // value is the policy value.
		urlToBookamrk string                       // url to be opened by the test
		testLogic     interface{}                  // business logic to be executed within test
	}{
		{
			name:          "unset",
			value:         &policy.EditBookmarksEnabled{Stat: policy.StatusUnset},
			urlToBookamrk: "https://google.com",
			testLogic:     unsetEditBookmarksBehavior,
		},
		{
			name:          "true",
			value:         &policy.EditBookmarksEnabled{Val: true},
			urlToBookamrk: "https://www.chromium.org/",
			testLogic:     bookmarksAllowedToEdit,
		},
		{
			name:          "false",
			value:         &policy.EditBookmarksEnabled{Val: false},
			urlToBookamrk: "https://abc.xyz",
			testLogic:     bookmarksNotEditable,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup
			if err := policyutil.ResetChrome(ctx, fakeDms, chrome); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// GIVEN
			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fakeDms, chrome, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			s.Logf("Open tab %s", param.urlToBookamrk)
			openedTab, err := chrome.NewConn(ctx, param.urlToBookamrk)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer openedTab.Close()

			// Run actual test
			runTest := param.testLogic.(func(ctx context.Context, s *testing.State))
			runTest(ctx, s)
		})
	}
}

func unsetEditBookmarksBehavior(ctx context.Context, s *testing.State) {
	// unset state leaves the bahavior of editing bookmarks enabled
	bookmarksAllowedToEdit(ctx, s)
}

func bookmarksAllowedToEdit(ctx context.Context, s *testing.State) {
	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	s.Log("Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	s.Log("Hit enter")
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	s.Log("Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	// WHEN
	newBookmarkName := "Custom bookmark1"
	s.Log("Change bookmark name to " + newBookmarkName)
	if err := keyboard.Type(ctx, newBookmarkName); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	s.Log("Hit enter")
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	s.Log("Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	chrome := s.PreValue().(*pre.PreData).Chrome
	testAPI, err := chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testAPI)

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find the Bookmark name textbox: ", err)
	}
	defer bookmarkNameTbx.Release(ctx)
	// THEN
	if bookmarkNameTbx.Value != newBookmarkName {
		s.Fatalf("Bookmark name was not different than expected. Expected: %s Actual: %s", newBookmarkName, bookmarkNameTbx.Value)
	}

	// AND
	s.Log("Bring up bookmark menu")
	removeBtn, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove",
	}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find the Remove button: ", err)
	}
	defer removeBtn.Release(ctx)

	s.Log("Click on Remove button")
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second}
	if err := removeBtn.StableLeftClick(ctx, &pollOpts); err != nil {
		s.Fatal("Failed to left click on the Remove button: ", err)
	}

	// THEN validate removing bookmark
	// Don't have an idea how to validate removal
}

func bookmarksNotEditable(ctx context.Context, s *testing.State) {
	chrome := s.PreValue().(*pre.PreData).Chrome
	testAPI, err := chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testAPI)
	// TODO: check exisitng bookmarks are still accessable

	// WHEN
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	s.Log("Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	// THEN
	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 2*time.Second)
	if err == nil {
		s.Fatal("Did not expect bookmark name text field to be visibale. Error: ", err)
	}
	defer bookmarkNameTbx.Release(ctx)
}
