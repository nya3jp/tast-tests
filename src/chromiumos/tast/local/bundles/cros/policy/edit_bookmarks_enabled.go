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
	cr := s.PreValue().(*pre.PreData).Chrome
	fakeDMS := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Enable bookmark editing for precondition
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, []policy.Policy{&policy.EditBookmarksEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}
	addInitialBookmark(ctx, tconn, cr)

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
			if err := policyutil.ResetChrome(ctx, fakeDMS, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fakeDMS, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			s.Logf("Open tab %s", param.urlToBookmark)
			conn, err := cr.NewConn(ctx, param.urlToBookmark)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if _, err := showBookmarkBar(ctx, tconn); err != nil {
				s.Fatal("Could not show bookmark bar: ", err)
			}

			// Run actual test.
			if result, err := canSeeBookmarkIcon(ctx, tconn); result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of checking visibility of bookmark icon: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			if result, err := canAddOpenedPageAsBookmark(ctx, tconn); result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of adding bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			if result, err := canRenameBookmark(ctx, tconn, param.bookmarkNewName); result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of editing bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			if result, err := canRemoveBookmark(ctx, tconn, param.bookmarkNewName); result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of removing bookmark: got %t, want %t. Err: %s", result, param.expectedResultOfInteraction, err)
			}
			if result, err := canOpenInitiallyAddedBookmark(ctx, tconn); result != true { // Opening already bookmarked pages is always allowed.
				s.Fatalf("Unexpected result of opening bookmark: got %t, want %t. Err: %s", result, true, err)
			}
		})
	}
}

func showBookmarkBar(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	visible, _ := ui.Exists(ctx, tconn, ui.FindParams{
		Name:      "Bookmarks",
		ClassName: "BookmarkBarView",
	})

	// Bar is already visible.
	if visible {
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

func addInitialBookmark(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (bool, error) {
	conn, err := cr.NewConn(ctx, "https://www.google.com/maps")
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome: ")
	}
	defer conn.Close()

	return canAddOpenedPageAsBookmark(ctx, tconn)
}

func canSeeBookmarkIcon(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark star icon is visible")
	result, err := ui.Exists(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Bookmark this tab",
	})
	if err != nil {
		return false, errors.Wrap(err, "unexpected existence of the bookmark star button: got true; want false: ")
	}
	return result, nil
}

func getBookmarksCount(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	bookmarks, err := ui.FindAll(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
	})
	if err != nil {
		return -1, errors.Wrap(err, "failed to find bookmarks on the bookmark bar: ")
	}
	return len(bookmarks), nil
}

func canAddOpenedPageAsBookmark(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be added")

	bookmarksCountBefore, err := getBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get Bookmarks buttons from the bookmark bar:")
	}

	if err := openBookmarkMenu(ctx, tconn); err != nil {
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

	toggleBookmarkBar(ctx)
	toggleBookmarkBar(ctx)

	bookmarksCountAfter, err := getBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get Bookmarks buttons from the bookmark bar:")
	}

	if bookmarksCountAfter != bookmarksCountBefore+1 {
		return false, errors.Errorf("unexpected bookmarks count. Got %d, want %d", bookmarksCountAfter, bookmarksCountBefore+1)
	}

	return true, nil
}

func canRenameBookmark(ctx context.Context, tconn *chrome.TestConn, newName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be edited")
	if err := openBookmarkMenu(ctx, tconn); err != nil {
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

	if err := openBookmarkMenu(ctx, tconn); err != nil {
		return false, err
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
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

func openBookmarkMenu(ctx context.Context, tconn *chrome.TestConn) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device: ")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Bring up bookmark menu")
	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		return errors.Wrap(err, "failed to write events")
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 10*time.Second)
	if err != nil {
		testing.ContextLog(ctx, "Couldn't open bookmark menu")
		return errors.Wrap(err, "failed to find bookmark name text field: ")
	}
	defer bookmarkNameTbx.Release(ctx)
	return nil
}

func canRemoveBookmark(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be removed")
	if err := openBookmarkMenu(ctx, tconn); err != nil {
		return false, err
	}

	removeBtn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
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

	// Toggle bar off/on to refresh it without a need to do any kind of polling
	toggleBookmarkBar(ctx)
	toggleBookmarkBar(ctx)

	visibleAfterRemoving, err := ui.Exists(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      bookmarkName,
	})
	if visibleAfterRemoving == true {
		return false, errors.New("unexpected visibility of the bookmark after removing it")
	}

	return true, nil
}

func getAddressBarTest(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	bookmarkedItem, err := ui.Find(ctx, tconn, ui.FindParams{
		ClassName: "OmniboxViewViews",
		Name:      "Address and search bar",
	})

	if err != nil {
		return "", errors.Wrap(err, "could not find address bar: ")
	}
	return bookmarkedItem.Value, nil
}

func canOpenInitiallyAddedBookmark(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be opened")
	bookmarkedItem, err := ui.Find(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      "Google Maps",
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to find Google Maps bookmark: ")
	}

	addressBefore, err := getAddressBarTest(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "problem getting address bar value: ")
	}

	bookmarkedItem.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second})

	addressAfter, err := getAddressBarTest(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "problem getting address bar value: ")
	}

	if addressAfter == addressBefore {
		return false, errors.Errorf("address bar value did not change after clicking on bookmark. Address value: %s", addressAfter)
	}

	return true, nil
}
