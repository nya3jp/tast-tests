// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googleapps

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// docsName represents the name of the Google Docs web area.
const docsName = "Google Docs"

var docWebArea = nodewith.NameContaining(docsName).Role(role.RootWebArea)

// NewGoogleDocs returns an action to create a new google document.
func NewGoogleDocs(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser, uiHandler cuj.UIActionHandler, newWindow bool) error {
	var opts []browser.CreateTargetOption
	if newWindow {
		opts = append(opts, browser.WithNewWindow())
	}
	testing.ContextLog(ctx, "Start to create google document")
	conn, err := uiHandler.NewChromeTab(ctx, br, cuj.NewGoogleDocsURL, newWindow)
	if err != nil {
		return errors.Wrap(err, "failed to open the google document")
	}
	defer conn.Close()
	return webutil.WaitForQuiescence(ctx, conn, longUITimeout)
}

// RenameDoc returns an action to rename the document.
func RenameDoc(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title string) action.Action {
	ui := uiauto.New(tconn)
	renameTextbox := nodewith.Name("Rename").ClassName("docs-title-input").Ancestor(docWebArea).Editable().Focusable()
	return ui.Retry(5, uiauto.NamedCombine("rename document",
		ui.WaitUntilExists(docWebArea),
		ui.LeftClickUntil(renameTextbox, ui.WithTimeout(5*time.Second).WaitUntilExists(renameTextbox.State("focused", true))),
		kb.AccelAction("Ctrl+A"),
		kb.TypeAction(title),
		waitForFieldTextToBe(tconn, renameTextbox, title),
		kb.AccelAction("Enter"),
		waitForDocsSaved(tconn),
	))
}

// EditDoc returns an action to edit the document.
func EditDoc(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, paragraph string) action.Action {
	ui := uiauto.New(tconn)
	content := nodewith.Name("Document content").Role(role.TextField).Ancestor(docWebArea).Editable().First()
	canvas := nodewith.Role(role.Canvas).Ancestor(docWebArea).First()
	return uiauto.NamedCombine("edit document",
		ui.WaitUntilExists(content),
		ui.LeftClick(canvas),
		kb.TypeAction(paragraph),
		waitForDocsSaved(tconn),
	)
}

// ChangeDocTextColor returns an action to change text color to specific color.
func ChangeDocTextColor(tconn *chrome.TestConn, color string) action.Action {
	ui := uiauto.New(tconn)
	moreButton := nodewith.Name("More").Role(role.ToggleButton).Ancestor(docWebArea)
	textColorButton := nodewith.Name("Text color").Role(role.PopUpButton).Ancestor(docWebArea)
	colorButton := nodewith.Name(color).Role(role.Cell).Ancestor(docWebArea)
	return uiauto.Retry(retryTimes, uiauto.NamedCombine("change document text color to "+color,
		uiauto.IfSuccessThen(ui.Gone(textColorButton), ui.LeftClickUntil(moreButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(textColorButton))),
		ui.LeftClickUntil(textColorButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(colorButton)),
		ui.LeftClick(colorButton),
		waitForDocsSaved(tconn),
	))
}

// ChangeDocFontSize returns an action to change font size to specific font size.
func ChangeDocFontSize(tconn *chrome.TestConn, size string) action.Action {
	ui := uiauto.New(tconn)
	moreButton := nodewith.Name("More").Role(role.ToggleButton).Ancestor(docWebArea)
	fontSizeTextField := nodewith.Name("Font size").Role(role.TextField).Ancestor(docWebArea)
	fontSizeOption18 := nodewith.Name(size).Role(role.ListBoxOption).Ancestor(docWebArea)
	return uiauto.Retry(retryTimes, uiauto.NamedCombine("change document font size to "+size,
		uiauto.IfSuccessThen(ui.Gone(fontSizeTextField), ui.LeftClickUntil(moreButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(fontSizeTextField))),
		ui.LeftClickUntil(fontSizeTextField, ui.WithTimeout(shortUITimeout).WaitUntilExists(fontSizeOption18)),
		ui.LeftClick(fontSizeOption18),
		waitForDocsSaved(tconn),
	))
}

// UndoDoc returns an action to undo document.
func UndoDoc(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	undoButton := nodewith.NameContaining("Undo").Role(role.Button).Ancestor(docWebArea)
	return uiauto.NamedCombine("undo document",
		ui.LeftClick(undoButton),
		waitForDocsSaved(tconn),
	)
}

// RedoDoc returns an action to redo document.
func RedoDoc(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	redoButton := nodewith.NameContaining("Redo").Role(role.Button).Ancestor(docWebArea)
	return uiauto.NamedCombine("redo document",
		ui.LeftClick(redoButton),
		waitForDocsSaved(tconn),
	)
}

// DeleteDoc returns an action to delete the document.
func DeleteDoc(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	docHomeWebArea := nodewith.Name(docsName).Role(role.RootWebArea)
	application := nodewith.Role(role.Application).Ancestor(docWebArea) // Google Docs appliction node.
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToDocsHome := nodewith.Name("Go to Docs home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	return uiauto.NamedCombine("delete document",
		cuj.ExpandMenu(tconn, fileButton, menu, 392),
		ui.DoDefault(moveToTrash),
		ui.DoDefault(goToDocsHome),
		// When leaving the edit document, sometimes the "Leave Site?" dialog box will pop up.
		// If it appears, click the leave button.
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(leaveButton),
			ui.DoDefaultUntil(leaveButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(leaveButton))),
		ui.WithTimeout(longUITimeout).WaitUntilExists(docHomeWebArea),
	)
}

// waitForDocsSaved waits for the docs document state to be saved.
func waitForDocsSaved(tconn *chrome.TestConn) action.Action {
	return waitForDocumentSaved(tconn, docsName)
}

// WaitUntilDocContentToBe waits for up to 5s until expected content.
func WaitUntilDocContentToBe(docConn *chrome.Conn, expectedContent string) action.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			docContent, err := docContent(ctx, docConn)
			if err != nil {
				return err
			}
			if docContent != expectedContent {
				return errors.Errorf("unexpected gdoc content: got %q; want %q", docContent, expectedContent)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second})
	}
}

func docContent(ctx context.Context, docConn *chrome.Conn) (content string, err error) {
	expr := `shadowPiercingQuery(".docs-texteventtarget-iframe").contentWindow.document.body.textContent`
	err = webutil.EvalWithShadowPiercer(ctx, docConn, expr, &content)
	return
}
