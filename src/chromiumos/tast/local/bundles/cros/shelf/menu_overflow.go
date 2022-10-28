// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MenuOverflow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the behavior of shelf menu when it is overflowed",
		Contacts: []string{
			"ting.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeApps",
	})
}

// MenuOverflow makes sure clicking left/right arrow is working when shelf overflows.
func MenuOverflow(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	if err := ash.EnterShelfOverflowWithFakeApps(ctx, tconn, false /* underRTL */); err != nil {
		s.Fatal("Failed to enter shelf overflow: ", err)
	}

	if err := ash.WaitForStableShelfBounds(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for stable shelf bounds: ", err)
	}

	if err := scrollEnd(ctx, tconn, scrollRight); err != nil {
		s.Fatal("Failed to scroll to right: ", err)
	}

	if err := scrollEnd(ctx, tconn, scrollLeft); err != nil {
		s.Fatal("Failed to scroll to left: ", err)
	}
}

// scrollDir specifies the scroll direction.
type scrollDir int

const (
	scrollLeft scrollDir = iota
	scrollRight
)

func scrollEnd(ctx context.Context, tconn *chrome.TestConn, d scrollDir) error {
	var scrolled bool
	timeout := 20 * time.Second

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Calculate the suitable scroll offset to go to a new shelf page.
		info, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return testing.PollBreak(err)
		}
		var pageOffset float32
		if d == scrollLeft {
			pageOffset = -info.PageOffset
		} else {
			pageOffset = info.PageOffset
		}

		// Calculate the target scroll offset based on pageOffset.
		if info, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{ScrollDistance: pageOffset}); err != nil {
			return testing.PollBreak(err)
		}

		// Choose the arrow button to be clicked based on the scroll direction.
		var arrowBounds coords.Rect
		if d == scrollLeft {
			arrowBounds = info.LeftArrowBounds
		} else {
			arrowBounds = info.RightArrowBounds
		}
		if arrowBounds.Width == 0 {
			// Have scrolled to the end. End polling.
			return nil
		}

		if err := ash.ScrollShelfAndWaitUntilFinish(ctx, tconn, arrowBounds, info.TargetMainAxisOffset); err != nil {
			return testing.PollBreak(err)
		}
		scrolled = true
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to scroll to end")
	}

	if !scrolled {
		return errors.Errorf("scroll animation haven't been triggered within %s", timeout)
	}

	return nil
}
