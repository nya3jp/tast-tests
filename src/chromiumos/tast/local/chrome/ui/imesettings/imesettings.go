// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imesettings supports managing input methods in OS settings.
package imesettings

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const inputsSubPageURL = "osLanguages/input"

var defaultIMESettingsPollOptions = &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

var addInputMethodButtonParams = ui.FindParams{
	Name: "Add input methods",
	Role: ui.RoleTypeButton,
}

var searchInputMethodFieldParams = ui.FindParams{
	Name: "Search by language or input name",
	Role: ui.RoleTypeSearchBox,
}

// ClickAddInputMethodButton clicks AddInputMethod button in inputs setting page.
// ClickAddInputMethodButton also waits for IME dialog to show up after click.
func ClickAddInputMethodButton(ctx context.Context, tconn *chrome.TestConn) error {
	return ui.StableFindAndClick(ctx, tconn, addInputMethodButtonParams, defaultIMESettingsPollOptions)
}

// SearchInputMethod searches input method by typing keyboard into searchbox.
// SearchInputMethod also waits for expected IME displayed on screen.
func SearchInputMethod(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, searchKeyword, inputMethodName string) error {
	if err := ui.StableFindAndClick(ctx, tconn, searchInputMethodFieldParams, defaultIMESettingsPollOptions); err != nil {
		return errors.Wrap(err, "failed to click input method search box")
	}

	if err := kb.Type(ctx, searchKeyword); err != nil {
		return errors.Wrapf(err, "failed to type %q on keyboard", searchKeyword)
	}

	// Wait for expected input method to show on screen.
	return ui.WaitUntilExists(ctx, tconn, ui.FindParams{
		Name:  inputMethodName,
		Role:  ui.RoleTypeCheckBox,
		State: map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 10*time.Second)
}

// SelectInputMethod selects an input method by displayed name.
func SelectInputMethod(ctx context.Context, tconn *chrome.TestConn, inputMethodName string) error {
	inputMethodOptionParams := ui.FindParams{
		Name: inputMethodName,
		Role: ui.RoleTypeCheckBox,
	}

	inputMethodNode, err := ossettings.DescendantNodeWithTimeout(ctx, tconn, inputMethodOptionParams, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find input method option: %s", inputMethodName)
	}
	defer inputMethodNode.Release(ctx)

	// Input method is possibly out of view port.
	if err := inputMethodNode.MakeVisible(ctx); err != nil {
		return errors.Wrap(err, "failed to make input method option visible")
	}

	return inputMethodNode.StableLeftClick(ctx, defaultIMESettingsPollOptions)
}

// ClickAddButtonToConfirm clicks Add button to confirm adding one or more input methods.
func ClickAddButtonToConfirm(ctx context.Context, tconn *chrome.TestConn) error {
	addButtonParams := ui.FindParams{
		Name: "Add",
		Role: ui.RoleTypeButton,
	}

	addButton, err := ossettings.DescendantNodeWithTimeout(ctx, tconn, addButtonParams, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find add button")
	}
	defer addButton.Release(ctx)

	return addButton.StableLeftClick(ctx, defaultIMESettingsPollOptions)
}

// RemoveInputMethod removes the input method by clicking cross button next to the input method on UI.
func RemoveInputMethod(ctx context.Context, tconn *chrome.TestConn, inputMethodName string) error {
	removeButtonParams := ui.FindParams{
		Name: "Remove " + inputMethodName,
		Role: ui.RoleTypeButton,
	}

	if err := ui.StableFindAndClick(ctx, tconn, removeButtonParams, defaultIMESettingsPollOptions); err != nil {
		return errors.Wrapf(err, "failed to click cross button to remove input method %q", inputMethodName)
	}
	return nil
}

// LaunchAtInputsSettingsPage launches Settings app at inputs setting page.
func LaunchAtInputsSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	condition := func(ctx context.Context) (bool, error) {
		return ossettings.DescendantNodeExists(ctx, tconn, ui.FindParams{Role: ui.RoleTypeHeading, Name: "Inputs"})
	}
	return ossettings.LaunchAtPageURL(ctx, tconn, cr, inputsSubPageURL, condition)
}
