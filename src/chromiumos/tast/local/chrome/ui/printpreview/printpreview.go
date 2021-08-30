// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printpreview provides support for controlling Chrome print preview
// directly through the UI.
package printpreview

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
)

// Layout represents the layout setting in Chrome print preview.
type Layout int

const (
	// Portrait represents the portrait layout setting.
	Portrait Layout = iota
	// Landscape represents the landscape layout setting.
	Landscape
)

// Print sets focus on the print button in Chrome print preview and injects the
// ENTER key to start printing. This is more reliable than clicking the print
// button since notifications often block it from view.
func Print(ctx context.Context, tconn *chrome.TestConn) error {
	printButton := nodewith.Name("Print").Role(role.Button)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("find and focus print button",
		ui.WithTimeout(10*time.Second).WaitUntilExists(printButton),
		ui.WithTimeout(10*time.Second).FocusAndWait(printButton),
	)(ctx); err != nil {
		return err
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}
	return nil
}

// SelectPrinter interacts with Chrome print preview to select the printer with
// the given printerName.
func SelectPrinter(ctx context.Context, tconn *chrome.TestConn, printerName string) error {
	// Find and expand the destination list.
	dataList := nodewith.Name("Destination Save as PDF").Role(role.PopUpButton)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("find and click destination list",
		ui.WithTimeout(10*time.Second).WaitUntilExists(dataList),
		ui.LeftClick(dataList),
	)(ctx); err != nil {
		return err
	}

	// Find and click the See more... menu item.
	seeMore := nodewith.Name("See more destinations").Role(role.MenuItem)
	if err := uiauto.Combine("find and click See more... menu item",
		ui.WithTimeout(10*time.Second).WaitUntilExists(seeMore),
		ui.LeftClick(seeMore),
	)(ctx); err != nil {
		return err
	}

	// Find and select the printer.
	printer := nodewith.Name(printerName).Role(role.Cell)
	if err := uiauto.Combine("find and click printer",
		ui.WithTimeout(10*time.Second).WaitUntilExists(printer),
		ui.LeftClick(printer),
	)(ctx); err != nil {
		return err
	}
	return nil
}

// SetLayout interacts with Chrome print preview to change the layout setting to
// the provided layout.
func SetLayout(ctx context.Context, tconn *chrome.TestConn, layout Layout) error {
	// Find and expand the layout list.
	layoutList := nodewith.Name("Layout").Role(role.PopUpButton)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("find and click layout list",
		ui.WithTimeout(10*time.Second).WaitUntilExists(layoutList),
		ui.LeftClick(layoutList),
	)(ctx); err != nil {
		return err
	}

	// Find the landscape layout option to verify the layout list has expanded.
	landscapeOption := nodewith.Name("Landscape").Role(role.ListBoxOption)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(landscapeOption)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for layout list to expand")
	}

	// Select the desired layout.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	var accelerator string
	switch layout {
	case Portrait:
		accelerator = "search+left"
	case Landscape:
		accelerator = "search+right"
	}
	if err := kb.Accel(ctx, accelerator); err != nil {
		return errors.Wrap(err, "failed to type accelerator")
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}
	return nil
}

// SetPages interacts with Chrome print preview to set the selected pages.
func SetPages(ctx context.Context, tconn *chrome.TestConn, pages string) error {
	// Find and expand the pages list.
	pageList := nodewith.Name("Pages").Role(role.PopUpButton)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("find and click page list",
		ui.WithTimeout(10*time.Second).WaitUntilExists(pageList),
		ui.LeftClick(pageList),
	)(ctx); err != nil {
		return err
	}

	// Find the custom pages option to verify the pages list has expanded.
	customOption := nodewith.Name("Custom").Role(role.ListBoxOption)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(customOption)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for pages list to expand")
	}

	// Select "Custom" and set the desired page range.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "search+right"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}
	// Wait for the custom pages text field to appear and become focused (this
	// happens automatically).
	textField := nodewith.Name("e.g. 1-5, 8, 11-13").Role(role.TextField).State(state.Focused, true)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(textField)(ctx); err != nil {
		return errors.Wrap(err, "failed to find custom pages text field")
	}
	if err := kb.Type(ctx, pages); err != nil {
		return errors.Wrap(err, "failed to type pages")
	}
	return nil
}

// WaitForPrintPreview waits for Print Preview to finish loading after it's
// initially opened.
func WaitForPrintPreview(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	loadingPreviewText := nodewith.Name("Loading preview")
	printPreviewFailedText := nodewith.Name("Print preview failed")
	return uiauto.Combine("wait for Print Preview to finish loading",
		// Wait for the loading text to appear to indicate print preview is loading.
		// Since print preview can finish loading before the loading text is found,
		// IfSuccessThen() is used with a nil "success" action just so that the
		// WaitUntilExists() error is ignored and won't fail the test.
		ui.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(loadingPreviewText), nil),
		// Wait for the loading text to be removed to indicate print preview is no
		// longer loading.
		ui.WithTimeout(30*time.Second).WaitUntilGone(loadingPreviewText),
		ui.Gone(printPreviewFailedText),
	)
}
