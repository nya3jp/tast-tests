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
	"chromiumos/tast/local/chrome/ui"
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
	params := ui.FindParams{
		Name: "Print",
		Role: ui.RoleTypeButton,
	}
	printButton, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find print button")
	}
	defer printButton.Release(ctx)
	if err := printButton.FocusAndWait(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed focusing print button")
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
	params := ui.FindParams{
		Name: "Destination Save as PDF",
		Role: ui.RoleTypePopUpButton,
	}
	destList, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find destination list")
	}
	defer destList.Release(ctx)
	if err := destList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click destination list")
	}

	// Find and click the See more... menu item.
	params = ui.FindParams{
		Name: "See more destinations",
		Role: ui.RoleTypeMenuItem,
	}
	seeMore, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find See more... menu item")
	}
	defer seeMore.Release(ctx)
	if err := seeMore.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click See more... menu item")
	}

	// Find and select the printer.
	params = ui.FindParams{
		Name: printerName,
		Role: ui.RoleTypeCell,
	}
	printer, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find printer")
	}
	defer printer.Release(ctx)
	if err := printer.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click printer")
	}
	return nil
}

// SetLayout interacts with Chrome print preview to change the layout setting to
// the provided layout.
func SetLayout(ctx context.Context, tconn *chrome.TestConn, layout Layout) error {
	// Find and expand the layout list.
	params := ui.FindParams{
		Name: "Layout",
		Role: ui.RoleTypePopUpButton,
	}
	layoutList, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find layout list")
	}
	defer layoutList.Release(ctx)
	if err := layoutList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click layout list")
	}

	// Find the landscape layout option to know the layout list has expanded.
	params = ui.FindParams{
		Name: "Landscape",
		Role: ui.RoleTypeListBoxOption,
	}
	layoutOption, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find landscape layout option")
	}
	defer layoutOption.Release(ctx)

	// Select the desired layout.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	var accelerator string
	switch layout {
	case Portrait:
		accelerator = "ctrl+alt+up"
	case Landscape:
		accelerator = "ctrl+alt+down"
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
	params := ui.FindParams{
		Name: "Pages",
		Role: ui.RoleTypePopUpButton,
	}
	pagesList, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find pages list")
	}
	defer pagesList.Release(ctx)
	if err := pagesList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click pages list")
	}

	// Find the custom pages option to know the pages list has expanded.
	params = ui.FindParams{
		Name: "Custom",
		Role: ui.RoleTypeListBoxOption,
	}
	customOption, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find custom pages option")
	}
	defer customOption.Release(ctx)

	// Select "Custom" and set the desired page range.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "ctrl+alt+down"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}
	// Wait for the custom pages text field to appear and become focused (this
	// happens automatically).
	params = ui.FindParams{
		Name:  "e.g. 1-5, 8, 11-13",
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeFocused: true},
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to find custom pages text field")
	}
	if err := kb.Type(ctx, pages); err != nil {
		return errors.Wrap(err, "failed to type pages")
	}
	return nil
}
