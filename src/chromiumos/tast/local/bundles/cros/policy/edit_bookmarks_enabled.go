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
	"chromiumos/tast/local/chrome/ui/browser/bookmarksbar"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

const nameOfInitialBookmark = "Alphabet"
const urlOfInitialBookmark = "https://abc.xyz"
const visibleAddressOfInitialBookmark = "abc.xyz"

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
	const errorMessage = "Encountered error when executing test logic for [%s], Err: %s"
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Enable bookmark editing for precondition.
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, []policy.Policy{&policy.EditBookmarksEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}
	addInitialBookmark(ctx, tconn, cr)

	for _, param := range []struct {
		name                        string                       // name is the name of the particular case checking policy value.
		value                       *policy.EditBookmarksEnabled // value is the policy value being tested.
		urlToBookmark               string                       // urlToBookmark is the url to be opened by the test.
		bookmarkNewName             string                       // bookmarkNewName is the name that will be used to rename bookmark.
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
			urlToBookmark:               "https://www.google.com/maps",
			bookmarkNewName:             "Custom bookmark 3", // Not relevant as this should not be possible. Added for consistency.
			expectedResultOfInteraction: false,
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

			if err := browser.ShowBookmarkBar(ctx, tconn); err != nil {
				s.Fatal("Could not show bookmark bar: ", err)
			}

			// Run actual test.
			testDomain := "checking visibility of bookmark icon"
			if result, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
				s.Fatalf(errorMessage, testDomain, err)
			} else if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of %s; got %t, want %t", testDomain, result, param.expectedResultOfInteraction)
			}

			testDomain = "adding bookmark"
			if result, err := canAddOpenedPageAsBookmark(ctx, tconn); err != nil {
				s.Fatalf(errorMessage, testDomain, err)
			} else if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of %s; got %t, want %t", testDomain, result, param.expectedResultOfInteraction)
			}

			testDomain = "editing bookmark"
			if result, err := canRenameBookmark(ctx, tconn, param.bookmarkNewName); err != nil {
				s.Fatalf(errorMessage, testDomain, err)
			} else if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of %s; got %t, want %t", testDomain, result, param.expectedResultOfInteraction)
			}

			testDomain = "removing bookmark"
			if result, err := canRemoveBookmark(ctx, tconn, param.bookmarkNewName); err != nil {
				s.Fatalf(errorMessage, testDomain, err)
			} else if result != param.expectedResultOfInteraction {
				s.Fatalf("Unexpected result of %s; got %t, want %t", testDomain, result, param.expectedResultOfInteraction)
			}

			testDomain = "opening existing bookmark"
			if result, err := canOpenInitiallyAddedBookmark(ctx, tconn); err != nil {
				s.Fatalf(errorMessage, testDomain, err)
			} else if result != true { // Opening already bookmarked pages is always allowed.
				s.Fatalf("Unexpected result of %s; got %t, want true", testDomain, result)
			}
		})
	}
}

func addInitialBookmark(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (bool, error) {
	conn, err := cr.NewConn(ctx, urlOfInitialBookmark)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()
	return canAddOpenedPageAsBookmark(ctx, tconn)
}

func canAddOpenedPageAsBookmark(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be added")
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	bookmarksCountBefore, err := bookmarksbar.GetVisibleBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if err := browser.AddCurrentTabAsBookmark(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not add current page as bookmark")
	}

	// Toggle bar off/on to refresh it without a need to do any kind of polling.
	for i := 0; i < 2; i++ {
		if err := browser.ToggleBookmarksBar(ctx); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	bookmarksCountAfter, err := bookmarksbar.GetVisibleBookmarksCount(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if bookmarksCountAfter != bookmarksCountBefore+1 {
		return false, errors.Errorf("unexpected bookmarks count. Got %d, want %d", bookmarksCountAfter, bookmarksCountBefore+1)
	}
	return true, nil
}

func canRenameBookmark(ctx context.Context, tconn *chrome.TestConn, newName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be edited")
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browser.RenameBookmarkForCurrentTab(ctx, tconn, newName); err != nil {
		return false, errors.Wrap(err, "could not rename the current tab bookmark")
	}

	bookmarkName, err := browser.GetCurrentTabBookmarkName(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Bookmark name")
	}

	if bookmarkName != newName {
		return false, errors.Wrapf(err, "unexpected bookmark name: got %s; want %s", newName, bookmarkName)
	}
	return true, nil
}

func canRemoveBookmark(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be removed")
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browser.IsBookmarkStarIconVisible(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browser.RemoveBookmarkForCurrentTab(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "could not remove bookmark")
	}

	// Toggle bar off/on to refresh it without a need to do any kind of polling.
	for i := 0; i < 2; i++ {
		if err := browser.ToggleBookmarksBar(ctx); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	visibleAfterRemoving, err := bookmarksbar.IsBookmarkVisible(ctx, tconn, bookmarkName)
	if err != nil {
		return false, errors.Wrapf(err, "could not check existence of bookmark %s", bookmarkName)
	}
	if visibleAfterRemoving == true {
		return false, errors.New("unexpected visibility of the bookmark after removing it")
	}
	return true, nil
}

func canOpenInitiallyAddedBookmark(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	testing.ContextLog(ctx, "Check if bookmark can be opened")
	if err := bookmarksbar.OpenBookmark(ctx, tconn, nameOfInitialBookmark); err != nil {
		return false, errors.Wrapf(err, "failed to open %s bookmark", nameOfInitialBookmark)
	}

	addressBarText, err := browser.GetAddressBarText(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "problem getting address bar value")
	}

	if addressBarText != visibleAddressForPreSetBookmark {
		return false, errors.Errorf("address bar value did not change after clicking on bookmark. Address bar value: %s", addressBarText)
	}
	return true, nil
}
