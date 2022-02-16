// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// MediaControlsPod is the 'Media controls' pod in the Quick Settings.
var MediaControlsPod = nodewith.NameStartingWith("Media controls").HasClass("Button").Ancestor(RootFinder)

// MediaControlsDetailView is the detailed Media controls view within the Quick Settings.
var MediaControlsDetailView = nodewith.HasClass("TrayDetailedView")

// PinnedMediaControls is the pinned 'Media controls' widget in shelf.
var PinnedMediaControls = nodewith.Role(role.Button).Name("Control your music, videos, and more").Ancestor(StatusAreaWidget)

// MediaControlsDialog is the opened 'Media controls' panel in shelf.
var MediaControlsDialog = nodewith.Role(role.Dialog).Name("Media controls").HasClass("RootView")

// PinMediaControlsPod pins the Media controls pod from the detail page.
func PinMediaControlsPod(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("click pin button and check the widget",
		ui.LeftClick(nodewith.Name("Pin to shelf").HasClass("IconButton").Ancestor(MediaControlsDetailView)),
		ui.WaitUntilExists(PinnedMediaControls),
	)
}

// UnpinMediaControlsPod unpins the media controls widget from shelf.
func UnpinMediaControlsPod(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("open media controls widget and unpin",
		ui.WithInterval(time.Second).LeftClickUntil(PinnedMediaControls, ui.Exists(MediaControlsDialog)),
		ui.LeftClick(nodewith.Role(role.Button).Name("Unpin").Ancestor(MediaControlsDialog)),
		ui.WaitUntilGone(PinnedMediaControls),
	)
}

// NavigateToMediaControlsSubpage navigates to the detailed Media controls view
// within the Quick Settings. This is safe to call even when the Quick Settings
// are already open.
func NavigateToMediaControlsSubpage(tconn *chrome.TestConn, title string) uiauto.Action {
	return func(ctx context.Context) error {
		if err := Expand(ctx, tconn); err != nil {
			return err
		}

		ui := uiauto.New(tconn)
		return uiauto.Combine("click the Media controls title",
			ui.LeftClick(nodewith.Name(title).HasClass("Label").Ancestor(MediaControlsPod)),
			ui.WaitUntilExists(MediaControlsDetailView),
		)(ctx)
	}
}
