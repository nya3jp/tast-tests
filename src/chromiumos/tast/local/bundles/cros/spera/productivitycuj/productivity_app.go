// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

const (
	// MicrosoftWeb indicates testing against Microsoft Web.
	MicrosoftWeb = "MicrosoftWeb"
	// GoogleWeb indicates testing against Google Web.
	GoogleWeb = "GoogleWeb"
)

const (
	// docText indicates content written as a paragraph of the "Microsoft Word" or "Google Docs".
	docText = "Copy to spreadsheet"
	// titleText indicates content written as a title of "Microsoft PowerPoint" or "Google Slides".
	titleText = "CUJ title"
	// subtitleText indicates content written as a subtitle of "Microsoft PowerPoint" or "Google Slides".
	subtitleText = "CUJ subtitle"
	// sheetText indicates content written as a cell of the "Microsoft Excel" or "Google Sheets".
	sheetText = "Copy to document"

	// sheetName indicates the name of the copied spreadsheet name.
	sheetName = "sum-sample"

	// rangeOfCells indicates the sum of rows in the spreadsheet.
	rangeOfCells = 100

	// dataWaitTime indicates the default time to wait for data-related operations.
	dataWaitTime = 2 * time.Second
	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
	// longerUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute

	// retryTimes defines the key UI operation retry times.
	retryTimes = 3
)

// ProductivityApp contains user's operation in productivity application.
type ProductivityApp interface {
	CreateDocument(ctx context.Context) error
	CreateSlides(ctx context.Context) error
	CreateSpreadsheet(ctx context.Context, cr *chrome.Chrome, sampleSheetURL, outDir string) (string, error)
	OpenSpreadsheet(ctx context.Context, filename string) error
	MoveDataFromDocToSheet(ctx context.Context) error
	MoveDataFromSheetToDoc(ctx context.Context) error
	ScrollPage(ctx context.Context) error
	SwitchToOfflineMode(ctx context.Context) error
	UpdateCells(ctx context.Context) error
	VoiceToTextTesting(ctx context.Context, expectedText string, playAudio action.Action) error
	Cleanup(ctx context.Context, sheetName string) error
	SetBrowser(br *browser.Browser)
}

// ProductivityParam defines the test parameters for productivity.
type ProductivityParam struct {
	Tier     cuj.Tier
	IsLacros bool
}

// dialogInfo holds the information of a dialog that will be encountered and needs to be handled during testing.
type dialogInfo struct {
	name string
	// dialog indicates the specified scene displayed.
	dialog *nodewith.Finder
	// node indicates the node displayed on the specified dialog.
	node *nodewith.Finder
}

// calculateSum calculates the sum of the "rangeOfCells" rows, but change one of the values (from "preVal" to "curVal").
func calculateSum(preVal, curVal int) int {
	sum := 0
	for i := 1; i <= rangeOfCells; i++ {
		if i == preVal {
			sum += curVal
		} else {
			sum += i
		}
	}
	return sum
}

// getClipboardText gets the clipboard text data.
func getClipboardText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var clipData string
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
		return "", err
	}
	return clipData, nil
}

// scrollTabPageByName scrolls the specified tab name of the webpage.
func scrollTabPageByName(ctx context.Context, uiHdl cuj.UIActionHandler, name string) error {
	scrollActions := uiHdl.ScrollChromePage(ctx)
	if err := uiHdl.SwitchToChromeTabByName(name)(ctx); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}
	for _, act := range scrollActions {
		if err := act(ctx); err != nil {
			return errors.Wrap(err, "failed to execute action")
		}
	}
	return nil
}
