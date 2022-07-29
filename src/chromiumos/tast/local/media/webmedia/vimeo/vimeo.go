// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vimeo

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/webmedia"
)

const shortTimeout = 5 * time.Second

// Vimeo holds resources to control Vimeo web page.
type Vimeo struct {
	*webmedia.Video
	tconn *chrome.TestConn
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
		tconn: tconn,
	}
}

// Play plays and verifies video playing.
func (v *Vimeo) Play(ctx context.Context) error {
	if err := v.Video.Play(ctx); err != nil {
		return err
	}

	ui := uiauto.New(v.tconn)
	promptDialog := nodewith.Role(role.Iframe).Name("Sign in with Google Dialog")

	if err := ui.WithTimeout(shortTimeout).WaitUntilGone(promptDialog)(ctx); err != nil {
		return ui.WithTimeout(30*time.Second).WithInterval(3*time.Second).RetryUntil(
			ui.WithTimeout(10*time.Second).DoDefault(nodewith.Name("Close").Role(role.Button).FinalAncestor(promptDialog)),
			ui.Gone(promptDialog),
		)(ctx)
	}
	return nil
}
