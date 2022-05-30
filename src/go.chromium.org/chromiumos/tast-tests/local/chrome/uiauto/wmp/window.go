// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmp contains utility functions for window resize operations.
package wmp

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/mouse"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/coords"
)

// ResizableArea setups resizeable area before performing resizing.
func ResizableArea(ctx context.Context, tconn *chrome.TestConn) (*coords.Rect, error) {
	ui := uiauto.New(tconn)

	rootWindowFinder := nodewith.HasClass("RootWindow-0").Role(role.Window)
	resizeArea, err := ui.Info(ctx, rootWindowFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root window info")
	}

	shelfInfo, err := ui.Info(ctx, nodewith.Role(role.Toolbar).ClassName("ShelfView"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shelf info")
	}
	resizeArea.Location.Height -= shelfInfo.Location.Height

	return &resizeArea.Location, nil
}

// StableDrag drags the window and waits for location be stabled.
func StableDrag(tconn *chrome.TestConn, window *nodewith.Finder, srcPt, endPt coords.Point) uiauto.Action {
	ui := uiauto.New(tconn)

	return uiauto.Combine("mouse drag and wait for location be stabled",
		ui.WaitForLocation(window),
		mouse.Drag(tconn, srcPt, endPt, time.Second),
		ui.WaitForLocation(window),
	)
}
