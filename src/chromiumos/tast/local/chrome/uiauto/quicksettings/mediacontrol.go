// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// MediaControlsPod is the 'Media controls' pod in the Quick Settings.
var MediaControlsPod = nodewith.NameStartingWith("Media controls").HasClass("Button").Ancestor(RootFinder)

// PinnedMediaControls is the pinned 'Media controls' widget in shelf.
var PinnedMediaControls = nodewith.Role(role.Button).Name("Control your music, videos, and more").Ancestor(StatusAreaWidget)

// MediaControlsDialog is the opened 'Media controls' panel in shelf.
var MediaControlsDialog = nodewith.Role(role.Dialog).Name("Media controls").HasClass("RootView")

// UnpinMediaControlsPod unpins the media controls widget from shelf.
func UnpinMediaControlsPod(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("open media controls widget and unpin",
		ui.LeftClick(PinnedMediaControls),
		ui.LeftClick(nodewith.Role(role.Button).Name("Unpin").Ancestor(MediaControlsDialog)),
		ui.WaitUntilGone(PinnedMediaControls),
	)
}
