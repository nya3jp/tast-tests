// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
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
		Func:         EditBookmarksEnabled,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Behavior of EditBookmarksEnabled policy: check if you can create, edit and remove bookmarks based on the policy value",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.EditBookmarksEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// EditBookmarksEnabled tests the EditBookmarksEnabaled policy for the enabled, disabled and unset cases.
func EditBookmarksEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
	if err := addInitialBookmark(ctx, tconn, cr, keyboard); err != nil {
		s.Fatal("Encountered error when adding initial bookmark: ", err)
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
			name:        "false",
			value:       &policy.EditBookmarksEnabled{Val: false},
			wantAllowed: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform Chrome reset.
			if err := policyutil.ResetChrome(ctx, fakeDMS, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fakeDMS, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			browserUI, err := browser.Launch(ctx, tconn, cr, param.urlToBookmark)
			if err != nil {
				s.Fatal("Failed to launch browser: ", err)
			}
			defer browserUI.Close(ctx)

			if err := browserUI.ShowBookmarksBar(ctx, keyboard); err != nil {
				s.Fatal("Could not show bookmark bar: ", err)
			}

			// Run actual test.
			if allowed, err := browserUI.IsBookmarkStarIconVisible(ctx); err != nil {
				s.Fatal("Encountered error when executing test logic for checking visibility of bookmark icon, Err: ", err)
			} else if allowed != param.wantAllowed {
				s.Errorf("Unexpected result for checking visibility of bookmark icon; got %t, want %t", allowed, param.wantAllowed)
			}

			if allowed, err := canAddBookmark(ctx, browserUI, keyboard); err != nil {
				s.Fatal("Encountered error when executing test logic for adding bookmark, Err: ", err)
			} else if allowed != param.wantAllowed {
				s.Errorf("Unexpected result for adding bookmark; got %t, want %t", allowed, param.wantAllowed)
			}

			if allowed, err := canRenameBookmark(ctx, browserUI, keyboard, param.bookmarkNewName); err != nil {
				s.Fatal("Encountered error when executing test logic for editing bookmark, Err: ", err)
			} else if allowed != param.wantAllowed {
				s.Errorf("Unexpected result for editing bookmark; got %t, want %t", allowed, param.wantAllowed)
			}

			if allowed, err := canRemoveBookmark(ctx, browserUI, keyboard, param.bookmarkNewName); err != nil {
				s.Fatal("Encountered error when executing test logic for removing bookmark, Err: ", err)
			} else if allowed != param.wantAllowed {
				s.Errorf("Unexpected result for removing bookmark; got %t, want %t", allowed, param.wantAllowed)
			}

			if allowed, err := canOpenInitiallyAddedBookmark(ctx, browserUI); err != nil {
				s.Fatal("Encountered error when executing test logic for opening existing bookmark, Err: ", err)
			} else if allowed != true { // Opening already bookmarked pages is always allowed.
				s.Errorf("Unexpected result for opening existing bookmark; got %t, want true", allowed)
			}
		})
	}
}

// addInitialBookmark adds initial bookmark.
func addInitialBookmark(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	browserUI, err := browser.Launch(ctx, tconn, cr, urlOfInitialBookmark)
	if err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}
	defer browserUI.Close(ctx)

	if err := browserUI.ShowBookmarksBar(ctx, keyboard); err != nil {
		return errors.Wrap(err, "could not show bookmarks bar")
	}

	if addedBm, err := canAddBookmark(ctx, browserUI, keyboard); err != nil {
		return errors.Wrap(err, "encountered error when adding bookmark")
	} else if addedBm != true {
		return errors.New("could not add bookmark")
	}
	return nil
}

// canAddBookmark attempts to add currently open tab as a bookmark and
// validates that operation on the bookmarks bar. Returns true if bookmarks
// count increased by one, otherwise returns false.
func canAddBookmark(ctx context.Context, browserUI *browser.Browser, keyboard *input.KeyboardEventWriter) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browserUI.IsBookmarkStarIconVisible(ctx); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	bookmarksCountBefore, err := browserUI.VisibleBookmarksCount(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if err := browserUI.BookmarkCurrentTab(ctx, keyboard); err != nil {
		return false, errors.Wrap(err, "could not add current page as bookmark")
	}

	// Toggle bar off/on to refresh it.
	for i := 0; i < 2; i++ {
		if err := browserUI.ToggleBookmarksBar(ctx, keyboard); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	bookmarksCountAfter, err := browserUI.VisibleBookmarksCount(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not get count of bookmarks")
	}

	if bookmarksCountAfter != bookmarksCountBefore+1 {
		return false, errors.Errorf("unexpected bookmarks count; got %d, want %d", bookmarksCountAfter, bookmarksCountBefore+1)
	}
	return true, nil
}

// canRenameBookmark attempts to rename bookmark's name for currently open tab
// Returns true if bookmark's name was changed, otherwise returns false.
func canRenameBookmark(ctx context.Context, browserUI *browser.Browser, keyboard *input.KeyboardEventWriter, newName string) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browserUI.IsBookmarkStarIconVisible(ctx); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browserUI.RenameBookmarkForCurrentTab(ctx, keyboard, newName); err != nil {
		return false, errors.Wrap(err, "could not rename the current tab bookmark")
	}

	bookmarkName, err := browserUI.CurrentTabBookmarkName(ctx, keyboard)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Bookmark name")
	}

	if bookmarkName != newName {
		return false, errors.Wrapf(err, "unexpected bookmark name; got %s; want %s", newName, bookmarkName)
	}
	return true, nil
}

// canRemoveBookmark attempts to remove bookmark for currently open tab.
// Validates removal operation by checking visibility of the bookmark on
// the bookmarks bar. Returns true if bookmark was removed, otherwise returns
// false.
func canRemoveBookmark(ctx context.Context, browserUI *browser.Browser, keyboard *input.KeyboardEventWriter, bookmarkName string) (bool, error) {
	// If we cannot see bookmark icon we stop execution of the test logic.
	// This relies on an assumption that visibility of the bookmark star icon
	// is related to bookmark functionality being enabled.
	if canSeeBookmarkIcon, err := browserUI.IsBookmarkStarIconVisible(ctx); err != nil {
		return false, errors.Wrap(err, "could not check if bookmark icon was visible")
	} else if canSeeBookmarkIcon != true {
		return false, nil
	}

	if err := browserUI.RemoveBookmarkForCurrentTab(ctx, keyboard); err != nil {
		return false, errors.Wrap(err, "could not remove bookmark")
	}

	// Toggle bar off/on to refresh it.
	for i := 0; i < 2; i++ {
		if err := browserUI.ToggleBookmarksBar(ctx, keyboard); err != nil {
			return false, errors.Wrap(err, "unable to toggle bookmark bar")
		}
	}

	visibleAfterRemoving, err := browserUI.IsBookmarkVisible(ctx, bookmarkName)
	if err != nil {
		return false, errors.Wrapf(err, "could not check existence of bookmark %s", bookmarkName)
	}
	if visibleAfterRemoving == true {
		return false, errors.New("unexpected visibility of the bookmark after removing it")
	}
	return true, nil
}

// canOpenInitiallyAddedBookmark attempts to open initially bookmarked page.
// Returns true if bookmark was opened, otherwise returns false.
func canOpenInitiallyAddedBookmark(ctx context.Context, browserUI *browser.Browser) (bool, error) {
	if err := browserUI.OpenBookmark(ctx, nameOfInitialBookmark); err != nil {
		return false, errors.Wrapf(err, "failed to open %s bookmark", nameOfInitialBookmark)
	}

	addressBarText, err := browserUI.GetAddressBarText(ctx)
	if err != nil {
		return false, errors.Wrap(err, "problem getting address bar value")
	}

	if addressBarText != visibleAddressOfInitialBookmark {
		return false, errors.Errorf("address bar value did not change after clicking on bookmark. Address bar value: %s", addressBarText)
	}
	return true, nil
}
