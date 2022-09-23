// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const clipboardHistoryContextMenuItemName = "Clipboard"
const clipboardHistoryTextItemViewClassName = "ClipboardHistoryTextItemView"
const contextMenuItemViewClassName = "MenuItemView"

// PasteAndVerify returns a function that pastes `text` from clipboard history
// into the field specified by `inputFinder`, replacing whatever text may have
// already been there, and verifies that the paste was successful.
func PasteAndVerify(ui *uiauto.Context, kb *input.KeyboardEventWriter,
	inputFinder *nodewith.Finder, contextMenu bool, pasteFn func(*uiauto.Context, *input.KeyboardEventWriter, *nodewith.Finder) uiauto.Action, text string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Paste %q", text)

		if err := uiauto.Combine("clear input field",
			ui.LeftClick(inputFinder),
			ui.WaitUntilExists(inputFinder),
			kb.AccelAction("Ctrl+A"),
			kb.AccelAction("Backspace"),
		)(ctx); err != nil {
			return err
		}

		nodeInfo, err := ui.Info(ctx, inputFinder)
		if err != nil {
			return errors.Wrap(err, "failed to get info for the input field")
		}

		if nodeInfo.Value != "" {
			return errors.Errorf("failed to clear value: %q", nodeInfo.Value)
		}

		var openClipboardHistory uiauto.Action
		if contextMenu {
			openClipboardHistory = openClipboardHistoryWithContextMenu(ui, inputFinder)
		} else {
			openClipboardHistory = openClipboardHistoryWithAccelerator(kb)
		}
		item := nodewith.Name(text).Role(role.MenuItem).HasClass(clipboardHistoryTextItemViewClassName).First()
		if err := uiauto.Combine(fmt.Sprintf("paste %q from clipboard history", text),
			openClipboardHistory,
			pasteFn(ui, kb, item),
			ui.WaitForLocation(inputFinder),
		)(ctx); err != nil {
			return err
		}

		nodeInfo, err = ui.Info(ctx, inputFinder)
		if err != nil {
			return errors.Wrap(err, "failed to get info for the input field")
		}

		if !strings.Contains(nodeInfo.Value, text) {
			return errors.Wrapf(nil, "input field didn't contain the word: got %q; want %q", nodeInfo.Value, text)
		}

		return nil
	}
}

func openClipboardHistoryWithContextMenu(ui *uiauto.Context, inputFinder *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("opening clipboard history with context menu",
		ui.RightClick(inputFinder),
		ui.DoDefault(nodewith.NameStartingWith(clipboardHistoryContextMenuItemName).Role(role.MenuItem)),
		ui.WaitUntilGone(nodewith.HasClass(contextMenuItemViewClassName)),
	)
}

func openClipboardHistoryWithAccelerator(kb *input.KeyboardEventWriter) uiauto.Action {
	return func(ctx context.Context) error {
		if err := kb.Accel(ctx, "Search+V"); err != nil {
			return errors.Wrap(err, "failed to launch clipboard history menu")
		}

		return nil
	}
}
