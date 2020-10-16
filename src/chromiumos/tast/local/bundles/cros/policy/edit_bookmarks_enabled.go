// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EditBookmarksEnabled,
		Desc: "Behavior of EditBookmarksEnabled policy: check if you can create, edit and remove bookmarks based on the policy value",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// EditBookmarksEnabled tests the EditBookmarksEnabaled policy for the enabled, disabled and unset cases.
func EditBookmarksEnabled(ctx context.Context, s *testing.State) {
	chromeInstance := s.PreValue().(*pre.PreData).Chrome
	fakeDms := s.PreValue().(*pre.PreData).FakeDMS

	testAPI, err := chromeInstance.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testAPI)

	// Enable bookmark editing for precondition
	if err := policyutil.ServeAndRefresh(ctx, fakeDms, chromeInstance, []policy.Policy{&policy.EditBookmarksEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}
	addInitialBookmark(ctx, testAPI, chromeInstance)

	for _, param := range []struct {
		name                        string                       // name is the name of the particular case checking policy value.
		value                       *policy.EditBookmarksEnabled // value is the policy value being tested.
		urlToBookmark               string                       // urlToBookmark is the url to be opened by the test.
		bookmarkNewName             string                       // bookmarkNewName is the name that will be used to rename bookmark
		expectedResultOfInteraction bool                         // expectedResultOfInteraction defines whether user should be able to add, edit, and delete bookmark.
	}{
		{
			name:                        "unset",
			value:                       &policy.EditBookmarksEnabled{Stat: policy.StatusUnset},
			urlToBookmark:               "https://google.com",
			bookmarkNewName:             "Custom bookmark 1",
			expectedResultOfInteraction: true,
		},
		{
			name:                        "true",
			value:                       &policy.EditBookmarksEnabled{Val: true},
			urlToBookmark:               "https://www.chromium.org/",
			bookmarkNewName:             "Custom bookmark 2",
			expectedResultOfInteraction: true,
		},
		{
			name:                        "false",
			value:                       &policy.EditBookmarksEnabled{Val: false},
			urlToBookmark:               "https://abc.xyz",
			bookmarkNewName:             "Custom bookmark 3", // Not relevant as this should not be possible. Added for consistency.
			expectedResultOfInteraction: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fakeDms, chromeInstance); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fakeDms, chromeInstance, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			s.Logf("Open tab %s", param.urlToBookmark)
			openedTab, err := chromeInstance.NewConn(ctx, param.urlToBookmark)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer openedTab.Close()

			if _, err := showBookmarkBar(ctx, testAPI); err != nil {
				s.Fatal("Could not show bookmark bar: ", err)
			}

			// Run actual test.
			result, err := canSeeBookmarkIcon(ctx, testAPI)
			if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of checking visibility of bookmark icon: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			result, err = canAddOpenedPageAsBookmark(ctx, testAPI)
			if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of adding bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			result, err = canRenameBookmark(ctx, testAPI, param.bookmarkNewName)
			if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of editing bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			result, err = canRemoveBookmark(ctx, testAPI, param.bookmarkNewName)
			if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of removing bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			result, err = canOpenInitiallyAddedBookmark(ctx, testAPI)
			if result != true { // Opening already bookmarked pages is always allowed.
				s.Fatalf("Unexpected result of opening bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
		})
	}
}

func showBookmarkBar(ctx context.Context, testAPI *chrome.TestConn) (bool, error) {
	result, _ := ui.Exists(ctx, testAPI, ui.FindParams{
		Name:      "Bookmarks",
		ClassName: "BookmarkBarView",
	})

	// Bar is already visible.
	if result == true {
		return true, nil
	}
	return toggleBookmarkBar(ctx)
}

func toggleBookmarkBar(ctx context.Context) (bool, error) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to open keyboard device: ")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Hit ctrl+shift+b")
	if err := keyboard.Accel(ctx, "ctrl+shift+b"); err != nil {
		return false, errors.Wrap(err, "failed to write events: ")
	}
	return true, nil
}

func addInitialBookmark(ctx context.Context, testAPI *chrome.TestConn, chromeInstance *chrome.Chrome) (bool, error) {
	openedTab, err := chromeInstance.NewConn(ctx, "https://www.google.com/maps")
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome: ")
	}
	defer openedTab.Close()

	return canAddOpenedPageAsBookmark(ctx, testAPI)
}

func canSeeBookmarkIcon(ctx context.Context, testAPI *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark star icon is visible")
	result, err := ui.Exists(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Bookmark this tab",
	})
	if err != nil {
		return false, errors.Wrap(err, "unexpected existence of the bookmark star button: got true; want false: ")
	}
	return result, nil
}

func canAddOpenedPageAsBookmark(ctx context.Context, testAPI *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be added")
	if err := openBookmarkMenu(ctx, testAPI); err != nil {
		return false, err
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to open keyboard device: ")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Hit enter")
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return false, errors.Wrap(err, "failed to write events: ")
	}
	return true, nil
}

func canRenameBookmark(ctx context.Context, testAPI *chrome.TestConn, newName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be edited")
	if err := openBookmarkMenu(ctx, testAPI); err != nil {
		return false, err
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to open keyboard device: ")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Change bookmark name to "+newName)
	if err := keyboard.Type(ctx, newName); err != nil {
		return false, errors.Wrap(err, "failed to write events: ")
	}

	testing.ContextLog(ctx, "Hit enter")
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return false, errors.Wrap(err, "failed to write events: ")
	}

	if err := openBookmarkMenu(ctx, testAPI); err != nil {
		return false, err
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 15*time.Second)
	if err != nil {
		return false, errors.Wrap(err, "failed to find the Bookmark name text field: ")
	}
	defer bookmarkNameTbx.Release(ctx)

	if bookmarkNameTbx.Value != newName {
		return false, errors.Wrapf(err, "unexpected bookmark name: got %s; want %s", newName, bookmarkNameTbx.Value)
	}

	testing.ContextLog(ctx, "Hit Escape")
	if err := keyboard.Accel(ctx, "Esc"); err != nil {
		return false, errors.Wrap(err, "failed to write events: ")
	}
	return true, nil
}

func openBookmarkMenu(ctx context.Context, testAPI *chrome.TestConn) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device: ")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		return errors.Wrap(err, "failed to write events")
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find bookmark name text field: ")
	}
	defer bookmarkNameTbx.Release(ctx)
	return nil
}

func canRemoveBookmark(ctx context.Context, testAPI *chrome.TestConn, bookmarkName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be removed")
	if err := openBookmarkMenu(ctx, testAPI); err != nil {
		return false, err
	}

	removeBtn, err := ui.FindWithTimeout(ctx, testAPI, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove",
	}, 5*time.Second)
	if err != nil {
		return false, errors.Wrap(err, "failed to find Remove button: ")
	}
	defer removeBtn.Release(ctx)

	testing.ContextLog(ctx, "Click on Remove button")
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second}
	if err := removeBtn.StableLeftClick(ctx, &pollOpts); err != nil {
		return false, errors.Wrap(err, "failed to left click on the Remove button: ")
	}

	toggleBookmarkBar(ctx)
	toggleBookmarkBar(ctx)

	visibleAfterRemoving, err := ui.Exists(ctx, testAPI, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      bookmarkName,
	})
	if visibleAfterRemoving == true {
		return false, errors.Wrap(err, "unexpected visibility of the bookmark after removing it: ")
	}

	return true, nil
}

func canOpenInitiallyAddedBookmark(ctx context.Context, testAPI *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be opened")
	bookmarkedItem, err := ui.Find(ctx, testAPI, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      "Google Maps",
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to find Google Maps bookmark: ")
	}

	bookmarkedItem.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second})

	// TODO: do the proper validation. How can we get e.g. url of the tab?

	return true, nil
}
