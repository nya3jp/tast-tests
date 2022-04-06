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
	"chromiumos/tast/local/chrome/uiauto/state"
)

// DragDrop drags the content specified from a source website to a Google search page.
func DragDrop(ctx context.Context, tconn *chrome.TestConn, content string) error {
	ui := uiauto.New(tconn)

	contentNode := nodewith.Name(content).First()
	start, err := ui.Location(ctx, contentNode)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for content")
	}

	searchTab := nodewith.Name("Search").Role(role.TextFieldWithComboBox).State(state.Editable, true).First()
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

// CheckDraggedContent checks if a certain |content| appears in the search box.
func CheckDraggedContent(ctx context.Context, ui *uiauto.Context, content string) error {
	contentNode := nodewith.NameContaining(content).Role(role.InlineTextBox).State(state.Editable, true).First()

	if err := ui.WaitUntilExists(contentNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to check for dragged content")
	}

	return nil
}
