// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// GoogleChat holds the information used to do Google Chat testing.
type GoogleChat struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
	uiHdl cuj.UIActionHandler
	kb    *input.KeyboardEventWriter
	conn  *chrome.Conn
}

// NewGoogleChat returns the the manager of Google Chat, caller will able to control Google Chat app through this object.
func NewGoogleChat(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, ui *uiauto.Context, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter) *GoogleChat {
	return &GoogleChat{
		cr:    cr,
		tconn: tconn,
		ui:    ui,
		uiHdl: uiHdl,
		kb:    kb,
	}
}

// maybeCloseWelcomeDialog closes the welcome dialog if it exists.
func (g *GoogleChat) maybeCloseWelcomeDialog(ctx context.Context) error {
	return nil
}

// Launch launches the Google Chat standalone app.
func (g *GoogleChat) Launch(ctx context.Context) (err error) {
	return nil
}

// StartChat starts a chat.
func (g *GoogleChat) StartChat(ctx context.Context) error {
	return nil
}
