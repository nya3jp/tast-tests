// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
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

	if err := g.login(ctx); err != nil {
		return "", errors.Wrap(err, "failed to login to the Chrome browser")
	}

	copy := nodewith.Name("Make a copy").Role(role.Button)
	if err := g.uiHdl.Click(copy)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to open the copied data spreadsheet")
	}

	timestamp := time.Now().Local().Format("2006-01-02")
	sheetName = fmt.Sprintf("%s-%s", sheetNamePrefix, timestamp)
	if err := g.renameFile(ctx, sheetName); err != nil {
		return "", err
	}
	g.sheetCreated = true
	return sheetName, nil
}

// CreatePivotTable creates the pivot table.
func (g *GoogleSheets) CreatePivotTable(ctx context.Context) error {
	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	insert := nodewith.Name("Insert").Role(role.MenuItem).Ancestor(menuBar)
	insertExpanded := insert.Expanded()
	pivotTable := nodewith.Name("Pivot table p").Role(role.MenuItem)
	createPivotDialog := nodewith.Name("Create pivot table").Role(role.Dialog)

	if err := uiauto.Combine("create pivot table",
		g.uiHdl.ClickUntil(insert, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(insertExpanded)),
		g.uiHdl.ClickUntil(pivotTable, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(createPivotDialog)),
	)(ctx); err != nil {
		return err
	}

	radioGroup := nodewith.Name("Insert to").Role(role.RadioGroup).Ancestor(createPivotDialog)
	existingSheet := nodewith.Name("Existing sheet").Role(role.RadioButton).Ancestor(radioGroup)
	pivotTableRange := nodewith.Name("e.g., Sheet1!F10").Role(role.TextField).Editable()
	pivotTableRangeFocused := pivotTableRange.Focused()
	createButton := nodewith.Name("Create").Role(role.Button).Ancestor(createPivotDialog)

	return uiauto.Combine("insert the pivot table to existing spreadsheet",
		g.fillInPivotRange,
		g.uiHdl.Click(existingSheet),
		g.uiHdl.ClickUntil(pivotTableRange, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(pivotTableRangeFocused)),
		g.kb.TypeAction(existingSheetRange),
		g.uiHdl.Click(createButton),
	)(ctx)
}

// EditPivotTable opens the pivot table editor and add settings.
func (g *GoogleSheets) EditPivotTable(ctx context.Context) error {
	pivotTableEditor := nodewith.Name("Pivot table editor").Role(role.Complementary)
	rowsAddButton := nodewith.Name("Rows Add").Role(role.PopUpButton).Ancestor(pivotTableEditor)
	rowsAddButtonExpanded := rowsAddButton.Expanded()
	buyerMenuItem := nodewith.Name("Buyer").Role(role.MenuItem)
	if err := uiauto.Combine("edit the rows",
		g.uiHdl.ClickUntil(rowsAddButton, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(rowsAddButtonExpanded)),
		g.uiHdl.Click(buyerMenuItem),
	)(ctx); err != nil {
		return err
	}

	columnsAddButton := nodewith.Name("Columns Add").Role(role.PopUpButton).Ancestor(pivotTableEditor)
	columnsAddButtonExpanded := columnsAddButton.Expanded()
	categoryItem := nodewith.Name("Category").Role(role.MenuItem)
	if err := uiauto.Combine("edit the columns",
		g.uiHdl.ClickUntil(columnsAddButton, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(columnsAddButtonExpanded)),
		g.uiHdl.Click(categoryItem),
	)(ctx); err != nil {
		return err
	}

	valuesAddButton := nodewith.Name("Values Add").Role(role.PopUpButton).Ancestor(pivotTableEditor)
	valuesAddButtonExpanded := valuesAddButton.Expanded()
	amountMenuItem := nodewith.Name("Amount").Role(role.MenuItem)
	if err := uiauto.Combine("edit the values",
		g.uiHdl.ClickUntil(valuesAddButton, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(valuesAddButtonExpanded)),
		g.uiHdl.Click(amountMenuItem),
	)(ctx); err != nil {
		return err
	}

	close := nodewith.Name("Close").Role(role.Button).Ancestor(pivotTableEditor)
	return uiauto.NamedAction("close the pivot table editor",
		g.ui.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(pivotTableEditor), g.uiHdl.Click(close)),
	)(ctx)
}

// ValidatePivotTable validates that the values in the pivot table meet our expectations.
func (g *GoogleSheets) ValidatePivotTable(ctx context.Context) error {
	// mapPivotTable defines the expected value of the total of each buyer.
	mapPivotTable := map[string]string{
		"J3": "172200",
		"J4": "124670",
		"J5": "133055",
	}

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

	g.conn.CloseTarget(ctx)
	g.conn.Close()
	return nil
}

// login logs in to the browser.
// Since we are now using a guest session, we need to log in to the browser.
func (g *GoogleSheets) login(ctx context.Context) error {
	account := nodewith.Name("電子郵件地址或電話號碼").Role(role.TextField).Editable()
	showPassword := nodewith.Name("顯示密碼").Role(role.CheckBox).Focusable()
	password := nodewith.Name("輸入您的密碼").Role(role.TextField).Editable()

	confirmInput := func(finder *nodewith.Finder, input string) uiauto.Action {
		return func(ctx context.Context) error {
			if err := g.kb.TypeAction(input)(ctx); err != nil {
				return err
			}
			return testing.Poll(ctx, func(ctx context.Context) error {
				node, err := g.ui.Info(ctx, finder)
				if err != nil {
					return err
				}
				if node.Value != input {
					if err := g.kb.AccelAction("Ctrl+A")(ctx); err != nil {
						return err
					}
					return errors.Errorf("%s is incorrect: got: %v; want: %v", node.Name, node.Value, input)
				}
				return nil
			}, &testing.PollOptions{Interval: time.Second, Timeout: 10 * time.Second})
		}
	}

	warning := nodewith.Name("保護您的帳戶").Role(role.StaticText)
	confirm := nodewith.Name("確認").Role(role.Button)

	return uiauto.Combine("login to browser",
		confirmInput(account, g.account),
		g.kb.AccelAction("Enter"),
		g.ui.WaitUntilExists(password),
		g.ui.LeftClick(showPassword),
		g.ui.LeftClick(password),
		confirmInput(password, g.password),
		g.kb.AccelAction("Enter"),
		g.ui.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(warning), g.uiHdl.Click(confirm)),
	)(ctx)
}

// maybeCloseWelcomeDialog closes the "Welcome to Google Sheets" dialog if it exists.
func (g *GoogleSheets) maybeCloseWelcomeDialog(ctx context.Context) error {
	welcomeDialog := nodewith.Name("Welcome to Google Sheets").Role(role.Dialog)
	closeButton := nodewith.Name("Close").Ancestor(welcomeDialog)
	return g.ui.IfSuccessThen(
		g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(welcomeDialog),
		g.uiHdl.Click(closeButton),
	)(ctx)
}

// validateEditMode checks if the share button exists to confirm whether to enter the edit mode.
func (g *GoogleSheets) validateEditMode(ctx context.Context) error {
	shareButton := nodewith.Name("Share. Private to only me. ").Role(role.Button)
	// Make sure share button exists to ensure to enter the edit mode. This is especially necessary on low-end DUTs.
	return g.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(shareButton)(ctx)
}

// openBlankDocument opens a blank document for the specified service.
func (g *GoogleSheets) openBlankDocument(ctx context.Context) error {
	blankOption := nodewith.Name("Blank").Role(role.ListBoxOption)
	return uiauto.Combine("open a blank document",
		g.maybeCloseWelcomeDialog,
		g.uiHdl.Click(blankOption),
		g.validateEditMode,
	)(ctx)
}

// renameFile renames the name of the spreadsheet.
func (g *GoogleSheets) renameFile(ctx context.Context, sheetName string) error {
	menuBar := nodewith.Name("Menu bar").Role(role.Banner)
	fileItem := nodewith.Name("File").Role(role.MenuItem).Ancestor(menuBar)
	fileMenu := nodewith.Role(role.Menu)
	renameItem := nodewith.Name("Rename r").Role(role.MenuItem).Ancestor(fileMenu)
	renameField := nodewith.Name("Rename").Role(role.TextField).Editable().Focused()
	return uiauto.Combine("rename the file",
		g.ui.WaitUntilExists(menuBar),
		g.uiHdl.ClickUntil(fileItem, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(fileMenu)),
		g.uiHdl.Click(renameItem),
		g.ui.WaitUntilExists(renameField),
		g.kb.TypeAction(sheetName),
		g.kb.AccelAction("Enter"),
		g.ui.Sleep(2*time.Second), // Wait Google Sheets to save the changes.
	)(ctx)
}

// removeFile removes the spreadsheet.
func (g *GoogleSheets) removeFile(ctx context.Context, sheetName *string) error {
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

// selectCell selects the specified cell using the name box.
func (g *GoogleSheets) selectCell(cell string) action.Action {
	nameBox := nodewith.Name("Name box (Ctrl + J)").Role(role.GenericContainer)
	nameField := nodewith.Role(role.TextField).FinalAncestor(nameBox)
	nameFieldFocused := nameField.Focused()
	return uiauto.NamedAction(fmt.Sprintf("to select cell %q", cell),
		uiauto.Combine("click the name box and name a range",
			g.uiHdl.ClickUntil(nameField, g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(nameFieldFocused)),
			g.kb.TypeAction(cell),
			g.kb.AccelAction("Enter"),
			// Given time to jump to the specific cell and select it.
			// And because we cannot be sure whether the target cell is focused, we have to wait a short time.
			g.ui.Sleep(500*time.Millisecond),
		),
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

// editCellValue edits the cell to the specified value.
func (g *GoogleSheets) editCellValue(ctx context.Context, cell, value string) error {
	testing.ContextLogf(ctx, "Writing cell %q value: %s", cell, value)

	return uiauto.Combine(fmt.Sprintf("write cell %q value", cell),
		g.selectCell(cell),
		g.kb.TypeAction(value),
		g.kb.AccelAction("Enter"),
	)(ctx)
}

// fillInPivotRange enters the pivot table data range in the range text box if it hasn't been filled in yet.
func (g *GoogleSheets) fillInPivotRange(ctx context.Context) error {
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

// Close closes the Google Sheets tab.
func (g *GoogleSheets) Close(ctx context.Context) {
	if g.conn == nil {
		return
	}
	g.conn.CloseTarget(ctx)
	g.conn.Close()
}
