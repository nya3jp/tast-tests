// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clipboard contains functionality shared by tests that
// exercise Clipboard restrictions of DLP.
package clipboard

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// CheckPasteNode checks if paste node exists.
func CheckPasteNode(ctx context.Context, ui *uiauto.Context) error {

	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)

	if err := ui.WaitUntilExists(pasteNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed: ")
	}

	return nil
}

// CheckClipboardBubble checks if clipboard restriction bubble exists.
func CheckClipboardBubble(ctx context.Context, ui *uiauto.Context, url string) error {

	bubbleView := nodewith.ClassName("ClipboardDlpBubble").Role(role.Window)
	bubbleClass := nodewith.ClassName("ClipboardBlockBubble").Ancestor(bubbleView)
	bubbleButton := nodewith.Name("Got it").Role(role.Button).Ancestor(bubbleClass)
	messageBlocked := "Pasting from " + url + " to this location is blocked by administrator policy"
	bubble := nodewith.Name(messageBlocked).Role(role.StaticText).Ancestor(bubbleClass)

	if err := uiauto.Combine("Bubble ",
		ui.WaitUntilExists(bubbleView),
		ui.WaitUntilExists(bubbleButton),
		ui.WaitUntilExists(bubbleClass),
		ui.WaitUntilExists(bubble))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for notification bubble existence: ")
	}

	return nil
}
