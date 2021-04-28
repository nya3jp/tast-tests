// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imesettings supports managing input methods in OS settings.
package imesettings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const inputsSubPageURL = "osLanguages/input"

var defaultPollOpts = testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

var addInputMethodButton = nodewith.Name("Add input methods").Role(role.Button)
var searchInputMethodField = nodewith.Name("Search by language or input name").Role(role.SearchBox)
var toggleShowInputOptionsInShelfFinder = nodewith.Name("Show input options in the shelf").Role(role.ToggleButton)

// IMESettings is a wrapper around the settings app used to control the inputs settings page.
type IMESettings struct {
	settings *ossettings.OSSettings
}

// LaunchAtInputsSettingsPage launches Settings app at inputs setting page.
func LaunchAtInputsSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*IMESettings, error) {
	ui := uiauto.New(tconn)
	inputsHeading := nodewith.Name("Inputs").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, inputsSubPageURL, ui.Exists(inputsHeading))
	if err != nil {
		return nil, err
	}
	return &IMESettings{settings: settings.WithPollOpts(defaultPollOpts)}, nil
}

// ClickAddInputMethodButton returns a function that clicks AddInputMethod button in inputs setting page.
func (i *IMESettings) ClickAddInputMethodButton() uiauto.Action {
	return i.settings.LeftClick(addInputMethodButton)
}

// SearchInputMethod returns a function that searches input method by typing keyboard into searchbox.
// SearchInputMethod also waits for expected IME displayed on screen.
func (i *IMESettings) SearchInputMethod(kb *input.KeyboardEventWriter, searchKeyword, inputMethodName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("SearchInputMethod(%s, %s)", searchKeyword, inputMethodName),
		i.settings.LeftClick(searchInputMethodField),
		kb.TypeAction(searchKeyword),
		i.settings.WaitUntilExists(nodewith.Name(inputMethodName).Role(role.CheckBox).Onscreen()),
	)
}

// SelectInputMethod returns a function that selects an input method by displayed name.
func (i *IMESettings) SelectInputMethod(inputMethodName string) uiauto.Action {
	inputMethodOption := nodewith.Name(inputMethodName).Role(role.CheckBox)
	return uiauto.Combine(fmt.Sprintf("SelectInputMethod(%s)", inputMethodName),
		i.settings.MakeVisible(inputMethodOption),
		i.settings.LeftClick(inputMethodOption),
	)
}

// ClickAddButtonToConfirm returns a function that clicks Add button to confirm adding one or more input methods.
func (i *IMESettings) ClickAddButtonToConfirm() uiauto.Action {
	return i.settings.LeftClick(nodewith.Name("Add").Role(role.Button))
}

// RemoveInputMethod returns a function that removes the input method by clicking cross button next to the input method on UI.
func (i *IMESettings) RemoveInputMethod(inputMethodName string) uiauto.Action {
	return i.settings.LeftClick(nodewith.Name("Remove " + inputMethodName).Role(role.Button))
}

// ToggleShowInputOptionsInShelf clicks toggle button of 'Show input options in the shelf' option.
func (i *IMESettings) ToggleShowInputOptionsInShelf() uiauto.Action {
	return i.settings.LeftClick(toggleShowInputOptionsInShelfFinder)
}
