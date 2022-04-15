// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googleapps

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// docsName represents the name of the Google Docs web area.
const docsName = "Google Docs"

// NewGoogleDocs returns an action to create a new google document.
func NewGoogleDocs(cs ash.ConnSource, tconn *chrome.TestConn, newWindow bool) action.Action {
	const newDocsURL = "https://docs.new"
	var opts []browser.CreateTargetOption
	if newWindow {
		opts = append(opts, browser.WithNewWindow())
	}
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start to create google document")
		conn, err := cs.NewConn(ctx, newDocsURL, opts...)
		if err != nil {
			return err
		}
		defer conn.Close()
		if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
			return errors.Wrap(err, "failed to wait for page to finish loading")
		}
		return nil
	}
}

// RenameDoc returns an action to rename the document.
func RenameDoc(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title string) action.Action {
	ui := uiauto.New(tconn)
	docWebArea := nodewith.NameContaining(docsName).Role(role.RootWebArea)
	renameTextbox := nodewith.Name("Rename").ClassName("docs-title-input").Ancestor(docWebArea).Editable().Focusable()
	return uiauto.NamedAction("to rename document",
		ui.Retry(5, uiauto.Combine("rename document",
			ui.WaitUntilExists(docWebArea),
			ui.LeftClickUntil(renameTextbox, ui.WithTimeout(5*time.Second).WaitUntilExists(renameTextbox.State("focused", true))),
			kb.AccelAction("Ctrl+A"),
			kb.TypeAction(title),
			waitForFieldTextToBe(tconn, renameTextbox, title),
			kb.AccelAction("Enter"),
			waitForDocsSaved(tconn),
		)),
	)
}

// EditDoc returns an action to edit the document.
func EditDoc(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, paragraph string) action.Action {
	ui := uiauto.New(tconn)
	docWebArea := nodewith.NameContaining(docsName).Role(role.RootWebArea)
	content := nodewith.Name("Document content").Role(role.TextField).Ancestor(docWebArea).Editable().First()
	return uiauto.NamedAction("to edit document",
		uiauto.Combine("edit document",
			ui.WaitUntilExists(content),
			kb.TypeAction(paragraph),
			waitForDocsSaved(tconn),
		),
	)
}

// DeleteDoc returns an action to delete the document.
func DeleteDoc(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	docWebArea := nodewith.NameContaining(docsName).Role(role.RootWebArea)
	docHomeWebArea := nodewith.Name(docsName).Role(role.RootWebArea)
	application := nodewith.Role(role.Application).Ancestor(docWebArea) // Google Docs appliction node.
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToDocsHome := nodewith.Name("Go to Docs home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	return uiauto.NamedAction("to delete document",
		uiauto.Combine("delete document",
			cuj.ExpandMenu(tconn, fileButton, menu, 392),
			ui.LeftClick(moveToTrash),
			ui.LeftClick(goToDocsHome),
			// When leaving the edit document, sometimes the "Leave Site?" dialog box will pop up.
			// If it appears, click the leave button.
			uiauto.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(leaveButton), ui.LeftClick(leaveButton)),
			ui.WithTimeout(longerUIWaitTime).WaitUntilExists(docHomeWebArea),
		),
	)
}

// waitForDocsSaved waits for the docs document state to be saved.
func waitForDocsSaved(tconn *chrome.TestConn) action.Action {
	return waitForDocumentSaved(tconn, docsName)
}
