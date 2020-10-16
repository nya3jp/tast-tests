// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/browser"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

// Constants needed to check if already bookmarked page can be accessed after applying policy.
const (
	nameOfInitialBookmark           = "Alphabet"        // displayed on the bookmarks bar
	urlOfInitialBookmark            = "https://abc.xyz" // used for navigation
	visibleAddressOfInitialBookmark = "abc.xyz"         // used in validation
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

	// Enable bookmark editing for allowing setup step - adding bookmark.
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, []policy.Policy{&policy.EditBookmarksEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	// Add initial bookmark.
	if allowed, err := addInitialBookmark(ctx, tconn, cr, keyboard); err != nil {
		s.Fatal("Encountered error when adding initial bookmark: ", err)
	} else if allowed != true {
		s.Fatalf("Unexpected result when adding initial bookmark; got %t, want true", allowed)
	}
	for _, param := range []struct {
		name            string                       // name is the name of the particular case checking policy value.
		value           *policy.EditBookmarksEnabled // value is the policy value being tested.
		urlToBookmark   string                       // urlToBookmark is the url to be opened by the test.
		bookmarkNewName string                       // bookmarkNewName is the name that will be used to rename bookmark.
		wantAllowed     bool                         // wantAllowed defines whether user should be able to add, edit, and delete bookmark.
	}{
		{
			name:            "unset",
			value:           &policy.EditBookmarksEnabled{Stat: policy.StatusUnset},
			urlToBookmark:   "https://google.com",
			bookmarkNewName: "Custom bookmark 1",
			wantAllowed:     true,
		},
		{
			name:            "true",
			value:           &policy.EditBookmarksEnabled{Val: true},
			urlToBookmark:   "https://www.chromium.org/",
			bookmarkNewName: "Custom bookmark 2",
			wantAllowed:     true,
		},
		{
			name:            "false",
			value:           &policy.EditBookmarksEnabled{Val: false},
			urlToBookmark:   "https://www.google.com/maps",
			bookmarkNewName: "Custom bookmark 3", // Not relevant as this should not be possible. Added for consistency.
			wantAllowed:     false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

			if err := browser.ShowBookmarksBar(ctx, tconn, keyboard); err != nil {
				s.Fatal("Could not show bookmark bar: ", err)
			}

			const assertionFailureErrorMessage = "Unexpected result of %s; got %t, want %t"
			// Run actual test.
			testStep := "checking visibility of bookmark icon"
			if allowed, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
				s.Fatalf("Encountered error when executing test logic for [%s], Err: %s", testStep, err)
			} else if allowed != param.wantAllowed {
				s.Fatalf("Unexpected result for %s; got %t, want %t", testStep, allowed, param.wantAllowed)
			}

			testStep = "adding bookmark"
			if allowed, err := canAddBookmark(ctx, tconn, keyboard); err != nil {
				s.Fatalf("Encountered error when executing test logic for [%s], Err: %s", testStep, err)
			} else if allowed != param.wantAllowed {
				s.Fatalf("Unexpected result for %s; got %t, want %t", testStep, allowed, param.wantAllowed)
			}

			testStep = "editing bookmark"
			if allowed, err := canRenameBookmark(ctx, tconn, keyboard, param.bookmarkNewName); err != nil {
				s.Fatalf("Encountered error when executing test logic for [%s], Err: %s", testStep, err)
			} else if allowed != param.wantAllowed {
				s.Fatalf("Unexpected result for %s; got %t, want %t", testStep, allowed, param.wantAllowed)
			}

			testStep = "removing bookmark"
			if allowed, err := canRemoveBookmark(ctx, tconn, keyboard, param.bookmarkNewName); err != nil {
				s.Fatalf("Encountered error when executing test logic for [%s], Err: %s", testStep, err)
			} else if allowed != param.wantAllowed {
				s.Fatalf("Unexpected result for %s; got %t, want %t", testStep, allowed, param.wantAllowed)
			}

			testStep = "opening existing bookmark"
			if allowed, err := canOpenInitiallyAddedBookmark(ctx, tconn); err != nil {
				s.Fatalf("Encountered error when executing test logic for [%s], Err: %s", testStep, err)
			} else if allowed != true { // Opening already bookmarked pages is always allowed.
				s.Fatalf("Unexpected result for %s; got %t, want true", testStep, allowed)
			}
		})
	}
}

func addInitialBookmark(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) (bool, error) {
	conn, err := cr.NewConn(ctx, urlOfInitialBookmark)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()

	if err := browser.ShowBookmarksBar(ctx, tconn, keyboard); err != nil {
		return false, errors.Wrap(err, "could not show bookmarks bar")
	}
	return canAddBookmark(ctx, tconn, keyboard)
}

// canAddBookmark attempts to add currently open tab as a bookmark and
// validates that operation on the bookmarks bar. Returns true if bookmarks
// count increased by one, otherwise returns false.
func canAddBookmark(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	bookmarksCountBefore, err := browser.VisibleBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if err := browser.BookmarkCurrentTab(ctx, tconn, keyboard); err != nil {
		return false, errors.Wrap(err, "could not add current page as bookmark")
	}

	// Toggle bar off/on to refresh it without a need to do any kind of polling.
	for i := 0; i < 2; i++ {
		if err := browser.ToggleBookmarksBar(ctx, keyboard); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	bookmarksCountAfter, err := browser.VisibleBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if bookmarksCountAfter != bookmarksCountBefore+1 {
		return false, errors.Errorf("unexpected bookmarks count; got %d, want %d", bookmarksCountAfter, bookmarksCountBefore+1)
	}
	return true, nil
}

func canRenameBookmark(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, newName string) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browser.RenameBookmarkForCurrentTab(ctx, tconn, keyboard, newName); err != nil {
		return false, errors.Wrap(err, "could not rename the current tab bookmark")
	}

	bookmarkName, err := browser.CurrentTabBookmarkName(ctx, tconn, keyboard)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Bookmark name")
	}

	if bookmarkName != newName {
		return false, errors.Wrapf(err, "unexpected bookmark name; got %s; want %s", newName, bookmarkName)
	}
	return true, nil
}

func canRemoveBookmark(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, bookmarkName string) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browser.RemoveBookmarkForCurrentTab(ctx, tconn, keyboard); err != nil {
		return false, errors.Wrap(err, "could not remove bookmark")
	}

	// Toggle bar off/on to refresh it without a need to do any kind of polling.
	for i := 0; i < 2; i++ {
		if err := browser.ToggleBookmarksBar(ctx, keyboard); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	visibleAfterRemoving, err := browser.IsBookmarkVisible(ctx, tconn, bookmarkName)
	if err != nil {
		return false, errors.Wrapf(err, "could not check existence of bookmark %s", bookmarkName)
	}
	if visibleAfterRemoving == true {
		return false, errors.New("unexpected visibility of the bookmark after removing it")
	}
	return true, nil
}

func canOpenInitiallyAddedBookmark(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	if err := browser.OpenBookmark(ctx, tconn, nameOfInitialBookmark); err != nil {
		return false, errors.Wrapf(err, "failed to open %s bookmark", nameOfInitialBookmark)
	}

	addressBarText, err := browser.GetAddressBarText(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "problem getting address bar value")
	}

	if addressBarText != visibleAddressOfInitialBookmark {
		return false, errors.Errorf("address bar value did not change after clicking on bookmark. Address bar value: %s", addressBarText)
	}
	return true, nil
}
