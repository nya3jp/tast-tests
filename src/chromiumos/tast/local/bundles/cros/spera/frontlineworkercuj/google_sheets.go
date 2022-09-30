// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// sheetNamePrefix indicates the prefix of the copied spreadsheet name.
	sheetNamePrefix = "pivot-sample"
	// existingSheetRange indicates the default spreadsheet range.
	existingSheetRange = "Sheet1!F1"
	// sheetsTab indicates the tab name of the "Google Sheets".
	sheetsTab = "Google Sheets"
	// rangeOfSampleData indicates the total number of rows in the sample spreadsheet.
	rangeOfSampleData = 10
	// rangeOfDataset indicates the total number of rows in the dataset spreadsheet.
	rangeOfDataset = 1000
)

// GoogleSheets holds the information used to do Google Sheets testing.
type GoogleSheets struct {
	cr           *chrome.Chrome
	tconn        *chrome.TestConn
	ui           *uiauto.Context
	uiHdl        cuj.UIActionHandler
	kb           *input.KeyboardEventWriter
	conn         *chrome.Conn
	account      string
	password     string
	sheetCreated bool
}

// NewGoogleSheets returns the the manager of Google Sheets, caller will able to control Google Sheets app through this object.
func NewGoogleSheets(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, ui *uiauto.Context,
	uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, account, password string) *GoogleSheets {
	return &GoogleSheets{
		cr:       cr,
		tconn:    tconn,
		ui:       ui,
		uiHdl:    uiHdl,
		kb:       kb,
		account:  account,
		password: password,
	}
}

// CopySpreadSheet copies the sample spreadsheet.
func (g *GoogleSheets) CopySpreadSheet(ctx context.Context, sampleSheetURL string) (sheetName string, err error) {
	g.conn, err = g.cr.NewConn(ctx, sampleSheetURL+"/copy")
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL: %s", sampleSheetURL)
	}

	if err := g.login()(ctx); err != nil {
		return "", errors.Wrap(err, "failed to login to the Chrome browser")
	}

	copy := nodewith.Name("Make a copy").Role(role.Button)
	if err := g.uiHdl.Click(copy)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to open the copied data spreadsheet")
	}

	if err := g.validateEditMode()(ctx); err != nil {
		return "", errors.Wrap(err, "failed to enter edit mode")
	}

	timestamp := time.Now().Local().Format("2006-01-02")
	sheetName = fmt.Sprintf("%s-%s", sheetNamePrefix, timestamp)
	if err := g.renameFile(sheetName)(ctx); err != nil {
		return "", err
	}
	g.sheetCreated = true
	return sheetName, nil
}

// CreatePivotTable creates the pivot table.
func (g *GoogleSheets) CreatePivotTable() uiauto.Action {
	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	insert := nodewith.Name("Insert").Role(role.MenuItem).Ancestor(menuBar)
	insertExpanded := insert.Expanded()
	pivotTable := nodewith.Name("Pivot table p").Role(role.MenuItem)
	createPivotDialog := nodewith.Name("Create pivot table").Role(role.Dialog)

	insertTable := uiauto.Combine("create pivot table",
		g.uiHdl.ClickUntil(insert, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(insertExpanded)),
		g.uiHdl.ClickUntil(pivotTable, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(createPivotDialog)),
	)

	radioGroup := nodewith.Name("Insert to").Role(role.RadioGroup).Ancestor(createPivotDialog)
	existingSheet := nodewith.Name("Existing sheet").Role(role.RadioButton).Ancestor(radioGroup)
	pivotTableRange := nodewith.Name("e.g., Sheet1!F10").Role(role.TextField).Editable()
	pivotTableRangeFocused := pivotTableRange.Focused()
	createButton := nodewith.Name("Create").Role(role.Button).Ancestor(createPivotDialog)
	return uiauto.Combine("insert the pivot table to existing spreadsheet",
		insertTable,
		g.fillInPivotRange(),
		g.uiHdl.Click(existingSheet),
		g.uiHdl.ClickUntil(pivotTableRange, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(pivotTableRangeFocused)),
		g.kb.TypeAction(existingSheetRange),
		g.uiHdl.Click(createButton),
	)
}

// EditPivotTable opens the pivot table editor and add settings.
func (g *GoogleSheets) EditPivotTable() uiauto.Action {
	const (
		rowsAddButtonName    = "Rows Add"
		rowName              = "Buyer"
		columnsAddButtonName = "Columns Add"
		columnName           = "Category"
		valuesAddButtonName  = "Values Add"
		valueName            = "Amount"
	)

	pivotTableEditor := nodewith.Name("Pivot table editor").Role(role.Complementary)
	close := nodewith.Name("Close").Role(role.Button).Ancestor(pivotTableEditor)
	return uiauto.NamedCombine("close the pivot table editor",
		g.editPivotTableEditor(rowsAddButtonName, rowName),
		g.editPivotTableEditor(columnsAddButtonName, columnName),
		g.editPivotTableEditor(valuesAddButtonName, valueName),
		uiauto.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(pivotTableEditor), g.uiHdl.Click(close)),
	)
}

func (g *GoogleSheets) editPivotTableEditor(buttonName, itemName string) uiauto.Action {
	pivotTableEditor := nodewith.Name("Pivot table editor").Role(role.Complementary)
	button := nodewith.Name(buttonName).Role(role.PopUpButton).Ancestor(pivotTableEditor)
	buttonCollapsed := button.Collapsed()
	buttonExpanded := button.Expanded()
	menuItem := nodewith.Name(itemName).Role(role.MenuItem)
	return uiauto.Combine("edit the values",
		// Sometimes, the display size is not large enough that when adding "Rows" and "Columns".
		// The "Value Add" button might drop outside of the display.
		uiauto.IfSuccessThen(
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(buttonCollapsed),
			g.ui.EnsureFocused(button),
		),
		g.uiHdl.ClickUntil(button, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(buttonExpanded)),
		g.uiHdl.Click(menuItem),
	)
}

// ValidatePivotTable validates that the values in the pivot table meet our expectations.
func (g *GoogleSheets) ValidatePivotTable() uiauto.Action {
	// mapPivotTable defines the expected value of the total of each buyer.
	mapPivotTable := map[string]string{
		"J3": "172200",
		"J4": "124670",
		"J5": "133055",
	}
	return func(ctx context.Context) error {
		// Calculate the total cost of each buyer.
		for i := 3; i < 6; i++ {
			grandTotal := 0
			for j := 0; j < 3; j++ {
				cell := fmt.Sprintf("%c%d", 'G'+j, i)
				v, err := g.getCellValue(ctx, cell)
				if err != nil {
					return err
				}
				value, err := strconv.Atoi(v)
				if err != nil {
					return err
				}
				grandTotal += value
			}
			totalAmountCell := fmt.Sprintf("J%d", i)
			expectedValue, err := strconv.Atoi(mapPivotTable[totalAmountCell])
			if err != nil {
				return err
			}
			if grandTotal != expectedValue {
				return errors.Errorf("the value of %q is incorrect; got: %d; want: %d", totalAmountCell, grandTotal, expectedValue)
			}
		}
		return nil
	}
}

// RemoveFile removes the spreadsheet.
func (g *GoogleSheets) RemoveFile(ctx context.Context, sheetName *string) error {
	if !g.sheetCreated {
		testing.ContextLog(ctx, "Spreadsheet was not successfully created")
		return nil
	}

	conn, err := g.cr.NewConn(ctx, cuj.GoogleSheetsURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleSheetsURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	fileOption := nodewith.NameContaining(*sheetName).Role(role.ListBoxOption).First()
	moreAction := nodewith.Name("More actions. Popup button.").Ancestor(fileOption)
	moreActionExpanded := moreAction.Expanded()
	remove := nodewith.Name("Remove").Role(role.MenuItem)
	moveToTrashDialog := nodewith.Name("Move to trash?").Role(role.Dialog)
	moveToTrashButton := nodewith.Name("MOVE TO TRASH").Role(role.Button).Ancestor(moveToTrashDialog)
	return uiauto.Combine("remove the spreadsheet",
		g.uiHdl.ClickUntil(moreAction, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(moreActionExpanded)),
		g.uiHdl.Click(remove),
		g.uiHdl.ClickUntil(moveToTrashButton, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(moveToTrashDialog)),
	)(ctx)
}

// login logs in to the browser.
// Since we are now using a guest session, we need to log in to the browser.
func (g *GoogleSheets) login() uiauto.Action {
	account := nodewith.Name("Email or phone").Role(role.TextField).Editable()
	showPassword := nodewith.Name("Show password").Role(role.CheckBox).Focusable()
	password := nodewith.Name("Enter your password").Role(role.TextField).Editable()

	confirmInput := func(finder *nodewith.Finder, input string) uiauto.Action {
		return func(ctx context.Context) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				if err := g.kb.TypeAction(input)(ctx); err != nil {
					return err
				}
				node, err := g.ui.Info(ctx, finder)
				if err != nil {
					return err
				}
				if node.Value == input {
					return nil
				}
				if err := g.kb.AccelAction("Ctrl+A")(ctx); err != nil {
					return err
				}
				return errors.Errorf("%s is incorrect: got: %v; want: %v", node.Name, node.Value, input)
			}, &testing.PollOptions{Timeout: 30 * time.Second})
		}
	}

	changeLanguage := func(ctx context.Context) error {
		heading := nodewith.Name("Sign in").Role(role.Heading)
		if err := g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(heading)(ctx); err == nil {
			return nil
		}
		changeLanguage := nodewith.Role(role.ListBox).Collapsed().Vertical()
		englishOption := nodewith.NameContaining("English (United States)").Role(role.ListBoxOption)
		return uiauto.NamedAction("change the language", uiauto.Combine("change the language",
			g.uiHdl.Click(changeLanguage),
			g.uiHdl.Click(englishOption),
			// The page might take longer to change the display to "English (United States)" language in low-end DUTs.
			g.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(account),
		))(ctx)
	}

	warning := nodewith.Name("Protect your account").Role(role.StaticText)
	confirm := nodewith.Name("CONFIRM").Role(role.Button).Focusable()

	return uiauto.Combine("login to browser",
		// Although the browser has been changed to English, the login page will still display another default language in low-end DUTs.
		changeLanguage,
		confirmInput(account, g.account),
		g.kb.AccelAction("Enter"),
		g.ui.WaitUntilExists(password),
		g.ui.LeftClick(showPassword),
		g.ui.LeftClick(password),
		confirmInput(password, g.password),
		g.kb.AccelAction("Enter"),
		uiauto.IfSuccessThen(g.ui.WaitUntilExists(warning), uiauto.NamedAction(`click the "CONFIRM" button`, g.uiHdl.Click(confirm))),
	)
}

// maybeCloseWelcomeDialog closes the "Welcome to Google Sheets" dialog if it exists.
func (g *GoogleSheets) maybeCloseWelcomeDialog() uiauto.Action {
	welcomeDialog := nodewith.Name("Welcome to Google Sheets").Role(role.Dialog)
	closeButton := nodewith.Name("Close").Ancestor(welcomeDialog)
	return uiauto.IfSuccessThen(
		g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(welcomeDialog),
		g.uiHdl.Click(closeButton),
	)
}

// validateEditMode checks if the share button exists to confirm whether to enter the edit mode.
func (g *GoogleSheets) validateEditMode() uiauto.Action {
	shareButton := nodewith.Name("Share. Private to only me. ").Role(role.Button)
	// Make sure share button exists to ensure to enter the edit mode. This is especially necessary on low-end DUTs.
	return g.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(shareButton)
}

// openBlankDocument opens a blank document for the specified service.
func (g *GoogleSheets) openBlankDocument() uiauto.Action {
	blankOption := nodewith.Name("Blank").Role(role.ListBoxOption)
	return uiauto.Combine("open a blank document",
		g.maybeCloseWelcomeDialog(),
		g.uiHdl.Click(blankOption),
		g.validateEditMode(),
	)
}

// renameFile renames the name of the spreadsheet.
func (g *GoogleSheets) renameFile(sheetName string) uiauto.Action {
	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	fileItem := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	fileMenu := nodewith.Role(role.Menu)
	renameItem := nodewith.Name("Rename r").Role(role.MenuItem).Ancestor(fileMenu)
	renameField := nodewith.Name("Rename").Role(role.TextField).Editable().Focused()
	closeButton := nodewith.Name("Close sidebar").Role(role.Button)
	return uiauto.Retry(3, uiauto.Combine("rename the file",
		g.ui.WaitUntilExists(menuBar),
		g.uiHdl.ClickUntil(fileItem, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileMenu)),
		g.uiHdl.Click(renameItem),
		// If the Approval sidebar appears, it means that the UI is still unstable, causing the error to click "Rename" to "Approvals".
		uiauto.IfSuccessThen(g.ui.WaitUntilExists(closeButton), g.uiHdl.Click(closeButton)),
		g.ui.WaitUntilExists(renameField),
		g.kb.TypeAction(sheetName),
		g.kb.AccelAction("Enter"),
		uiauto.Sleep(2*time.Second), // Wait Google Sheets to save the changes.
	))
}

// selectCell selects the specified cell using the name box.
func (g *GoogleSheets) selectCell(cell string) uiauto.Action {
	nameBox := nodewith.Name("Name box (Ctrl + J)").Role(role.GenericContainer)
	nameField := nodewith.Role(role.TextField).FinalAncestor(nameBox)
	nameFieldFocused := nameField.Focused()
	return uiauto.NamedCombine(fmt.Sprintf("select cell %q", cell),
		g.uiHdl.ClickUntil(nameField, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(nameFieldFocused)),
		g.kb.TypeAction(cell),
		g.kb.AccelAction("Enter"),
		// Given time to jump to the specific cell and select it.
		// And because we cannot be sure whether the target cell is focused, we have to wait a short time.
		uiauto.Sleep(500*time.Millisecond),
	)
}

// getClipboardText gets the clipboard text data.
func getClipboardText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var clipData string
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
		return "", err
	}
	return clipData, nil
}

// getCellValue gets the value of the specified cell.
func (g *GoogleSheets) getCellValue(ctx context.Context, cell string) (clipData string, err error) {
	if err := g.selectCell(cell)(ctx); err != nil {
		return "", err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Due to the unstable network, there might be no data in the clipboard after the copy operation.
		// Therefore, we also need to retry the copy operation.
		if err := g.kb.AccelAction("Ctrl+C")(ctx); err != nil {
			return err
		}
		clipData, err = getClipboardText(ctx, g.tconn)
		if err != nil {
			return err
		}
		if clipData == "Retrieving data. Wait a few seconds and try to cut or copy again." {
			return errors.New("clipboard data is not yet ready")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
		return "", err
	}

	testing.ContextLogf(ctx, "Getting cell %q value: %s", cell, clipData)

	return clipData, nil
}

// fillInPivotRange enters the pivot table data range in the range text box if it hasn't been filled in yet.
func (g *GoogleSheets) fillInPivotRange() uiauto.Action {
	return func(ctx context.Context) error {
		createPivotDialog := nodewith.Name("Create pivot table").Role(role.Dialog)
		rangeTextField := nodewith.Name("Data range").Role(role.TextField).Editable().Ancestor(createPivotDialog)
		info, err := g.ui.Info(ctx, rangeTextField)
		if err != nil {
			return err
		}
		dataRange := fmt.Sprintf("Sheet1!A%d:C%d", 1, rangeOfDataset+1)
		if info.Value != dataRange {
			return uiauto.Combine("rewrite the data range",
				g.uiHdl.Click(rangeTextField),
				g.kb.AccelAction("Ctrl+A"),
				g.kb.TypeAction(dataRange),
			)(ctx)
		}
		// If it has filled in the correct data range information, skip entering it again.
		testing.ContextLog(ctx, "Data range information is correct")
		return nil
	}
}

// Close closes the Google Sheets tab.
func (g *GoogleSheets) Close(ctx context.Context) {
	if g.conn == nil {
		return
	}
	g.conn.CloseTarget(ctx)
	g.conn.Close()
}
