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

// Layout represents the layout setting in Chrome print preview.
type Layout int

const (
	// Portrait represents the portrait layout setting.
	Portrait Layout = iota
	// Landscape represents the landscape layout setting.
	Landscape
)

// Print clicks the print button in Chrome print preview.
func Print(ctx context.Context, root *ui.Node) error {
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

	// Select "See more..." to get the complete list of printers.
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

// SetLayout interacts with Chrome print preview to change the layout setting to
// the provided layout.
func SetLayout(ctx context.Context, root *ui.Node, layout Layout) error {
	// Find and expand the layout list.
	params := ui.FindParams{
		Name: "Layout",
		Role: ui.RoleTypePopUpButton,
	}
	layoutList, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find layout list")
	}
	defer layoutList.Release(ctx)
	if err := layoutList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click layout list")
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
		accelerator = "home"
	case Landscape:
		accelerator = "end"
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
func SetPages(ctx context.Context, root *ui.Node, pages string) error {
	// Find and expand the pages list.
	params := ui.FindParams{
		Name: "Pages",
		Role: ui.RoleTypePopUpButton,
	}
	pagesList, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find pages list")
	}
	defer pagesList.Release(ctx)
	if err := pagesList.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click pages list")
	}

	// Select "Custom" and set the desired page range.
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
	params = ui.FindParams{
		Name: "e.g. 1-5, 8, 11-13",
		Role: ui.RoleTypeTextField,
	}
	pagesField, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find custom pages text field")
	}
	defer pagesField.Release(ctx)
	if err := pagesField.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click custom pages text field")
	}
	if err := kb.Type(ctx, pages); err != nil {
		return errors.Wrap(err, "failed to type pages")
	}
	return nil
}
