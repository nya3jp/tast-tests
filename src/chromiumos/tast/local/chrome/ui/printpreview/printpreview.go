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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
)

// Print clicks the print button in Chrome print preview.
func Print(ctx context.Context, root *ui.Node) error {
	// Find and click the print button.
	params := ui.FindParams{
		Name: "Print",
		Role: ui.RoleTypeButton,
	}
	printButton, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find print button")
	}
	defer printButton.Release(ctx)
	if err := printButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click print button")
	}
	return nil
}

// SelectPrinter interacts with Chrome print preview to select the printer with
// the given printerName.
func SelectPrinter(ctx context.Context, root *ui.Node, printerName string) error {
	// Find and expand the destination list.
	params := ui.FindParams{
		Role: ui.RoleTypePopUpButton,
	}
	destList, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find destination list")
	}
	defer destList.Release(ctx)
	if err := destList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click destination list")
	}

	// Find and click "See more..." to get the list of printers.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "end"); err != nil {
		return errors.Wrap(err, "failed to type end")
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	// Find and select the printer.
	params = ui.FindParams{
		Name: printerName,
		Role: ui.RoleTypeCell,
	}
	printer, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find printer")
	}
	defer printer.Release(ctx)
	if err := printer.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click printer")
	}
	return nil
}
