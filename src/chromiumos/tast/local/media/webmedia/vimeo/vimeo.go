// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vimeo

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/webmedia"
)

// Vimeo holds resources to control Vimeo web page.
type Vimeo struct {
	*webmedia.Video
}

// New returns a new Vimeo instance.
func New(tconn *chrome.TestConn, url string) *Vimeo {
	return &Vimeo{
		Video: webmedia.New(
			tconn,
			url,
			"document.querySelector('video')",
			nodewith.Role(role.Video).Ancestor(nodewith.Role(role.Window).NameContaining("Vimeo").HasClass("BrowserFrame")),
		),
	}
}
