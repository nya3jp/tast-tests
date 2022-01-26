// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clipboard contains functionality shared by tests that
// exercise Clipboard restrictions of DLP.
package clipboard

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// CheckGreyPasteNode checks if greyed paste node exists.
func CheckGreyPasteNode(ctx context.Context, ui *uiauto.Context) error {
	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)
	pasteActiveNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem).State("focusable", true)

	if err := uiauto.Combine("Check paste node greyed ",
		ui.WaitUntilExists(pasteNode),
		ui.WaitUntilGone(pasteActiveNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed")
	}

	return nil
}

// CheckClipboardBubble checks if clipboard restriction bubble exists.
func CheckClipboardBubble(ctx context.Context, ui *uiauto.Context, url string) error {
	// Message name - IDS_POLICY_DLP_CLIPBOARD_BLOCKED_ON_PASTE
	bubbleView := nodewith.ClassName("ClipboardDlpBubble").Role(role.Window)
	bubbleClass := nodewith.ClassName("ClipboardBlockBubble").Ancestor(bubbleView)
	bubbleButton := nodewith.Name("Got it").Role(role.Button).Ancestor(bubbleClass)
	messageBlocked := "Pasting from " + url + " to this location is blocked by administrator policy. Learn more"
	bubble := nodewith.Name(messageBlocked).Role(role.StaticText).Ancestor(bubbleClass)

	if err := uiauto.Combine("Bubble ",
		ui.WaitUntilExists(bubbleView),
		ui.WaitUntilExists(bubbleButton),
		ui.WaitUntilExists(bubble))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for notification bubble existence")
	}

	return nil
}

// GetClipboardContent retrieves the current clipboard content.
func GetClipboardContent(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var clipData string
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
		return "", errors.Wrap(err, "failed to get clipboard content")
	}
	return clipData, nil
}

// CheckPastedContent checks if a certain string appears in the search box
func CheckPastedContent(ctx context.Context, ui *uiauto.Context, content string) error {
	// Slicing the string to get first 10 words in single line.
	// Since pasted string in search box will be in single line format.
	words := strings.Fields(content)
	content = strings.Join(words[:10], " ")

	contentNode := nodewith.NameContaining(content).Role(role.InlineTextBox).First()

	if err := uiauto.Combine("Pasted ",
		ui.WaitUntilExists(contentNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for pasted content")
	}

	return nil
}
