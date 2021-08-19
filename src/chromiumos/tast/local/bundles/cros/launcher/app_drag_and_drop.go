// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/launcher/draganddrop"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppDragAndDrop,
		Desc:     "Test the functionality of dragging and dropping on app icons",
		Contacts: []string{"kyle.chen@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "chromeLoggedIn",
	})
}

// AppDragAndDrop tests the functionality of dragging and dropping on app icons.
func AppDragAndDrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := dragandDrop(ctx, tconn, cr, false); err != nil {
		s.Fatal("Failed to drag and drop on app icons by mouse: ", err)
	}
	if err := dragandDrop(ctx, tconn, cr, true); err != nil {
		s.Fatal("Failed to drag and drop on app icons by touch: ", err)
	}
}

// dragandDrop tests the functionality of dragging and dropping on app icons in different ways.
func dragandDrop(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, isTouch bool) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTouch)
	if err != nil {
		return errors.Wrapf(err, "failed to set tablet mode to be %v", isTouch)
	}
	defer cleanup(cleanupCtx)

	var handler draganddrop.DragAndDrop
	if isTouch {
		if handler, err = draganddrop.NewTouchHandler(ctx, cr, tconn); err != nil {
			return errors.Wrap(err, "failed to new a interface by touch")
		}
	} else {
		if handler, err = draganddrop.NewMouseHandler(ctx, cr, tconn); err != nil {
			return errors.Wrap(err, "failed to new a interface by mouse")
		}
	}
	defer handler.Close()

	if err := launcher.Open(tconn)(ctx); err != nil {
		return errors.Wrap(err, "failed to open the launcher")
	}

	if err := handler.DragFirstIconToThirdIcon(ctx); err != nil {
		return errors.Wrap(err, "failed to drag icon to center")
	}

	if err := handler.DragFirstIconToNextPage(ctx); err != nil {
		return errors.Wrap(err, "failed to drag icon to next page")
	}

	if err := handler.VerifySecondPage(ctx); err != nil {
		return errors.Wrap(err, "failed to return the first page")
	}

	return nil
}
