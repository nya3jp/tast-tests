// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dragdrop contains functionality shared by tests that
// exercise Drag and Drop restrictions of DLP.
package dragdrop

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// DragDrop drags the content specified from a source website to a Google seach page.
func DragDrop(ctx context.Context, tconn *chrome.TestConn, content string) error {
	ui := uiauto.New(tconn)

	contentNode := nodewith.Name(content).First()
	start, err := ui.Location(ctx, contentNode)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for content")
	}

	search := "Google Search"
	searchTab := nodewith.Name(search).Role(role.InlineTextBox).First()
	endLocation, err := ui.Location(ctx, searchTab)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for google search")
	}

	if err := uiauto.Combine("Drag and Drop",
		mouse.Drag(tconn, start.CenterPoint(), endLocation.CenterPoint(), time.Second*2))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify content preview for")
	}

	return nil
}
