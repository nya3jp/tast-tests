// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imesettings supports managing input methods in OS settings.
package imesettings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
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

// SettingOption represents an IME setting item shown in OS Settings.
type settingOption string

// Available IME setting items.
const (
	GlideTyping             settingOption = "Enable glide typing"
	AutoCapitalization      settingOption = "Auto-capitalization"
	ShowInputOptionsInShelf settingOption = "Show input options in the shelf"
)

// IMESettings is a wrapper around the settings app used to control the inputs settings page.
type IMESettings struct {
	*ossettings.OSSettings
}

// New returns a new IME settings context.
func New(tconn *chrome.TestConn) *IMESettings {
	return &IMESettings{ossettings.New(tconn)}
}

// LaunchAtInputsSettingsPage launches Settings app at inputs setting page.
func LaunchAtInputsSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*IMESettings, error) {
	ui := uiauto.New(tconn)
	inputsHeading := nodewith.NameStartingWith("Inputs").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, inputsSubPageURL, ui.Exists(inputsHeading))
	if err != nil {
		return nil, err
	}
	return &IMESettings{settings.WithPollOpts(defaultPollOpts)}, nil
}

// ClickAddInputMethodButton returns a function that clicks AddInputMethod button in inputs setting page.
func (i *IMESettings) ClickAddInputMethodButton() uiauto.Action {
	return i.LeftClick(addInputMethodButton)
}

// SearchInputMethod returns a function that searches input method by typing keyboard into searchbox.
// SearchInputMethod also waits for expected IME displayed on screen.
func (i *IMESettings) SearchInputMethod(kb *input.KeyboardEventWriter, searchKeyword, inputMethodName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("SearchInputMethod(%s, %s)", searchKeyword, inputMethodName),
		i.LeftClick(searchInputMethodField),
		kb.TypeAction(searchKeyword),
		i.WaitUntilExists(nodewith.Name(inputMethodName).Role(role.CheckBox).Onscreen()),
	)
}

// SelectInputMethod returns a function that selects an input method by displayed name.
func (i *IMESettings) SelectInputMethod(inputMethodName string) uiauto.Action {
	inputMethodOption := nodewith.Name(inputMethodName).Role(role.CheckBox)
	return uiauto.Combine(fmt.Sprintf("SelectInputMethod(%s)", inputMethodName),
		i.MakeVisible(inputMethodOption),
		i.LeftClick(inputMethodOption),
	)
}

// ClickAddButtonToConfirm returns a function that clicks Add button to confirm adding one or more input methods.
func (i *IMESettings) ClickAddButtonToConfirm() uiauto.Action {
	return i.LeftClick(nodewith.Name("Add").Role(role.Button))
}

// RemoveInputMethod returns a function that removes the input method by clicking cross button next to the input method on UI.
func (i *IMESettings) RemoveInputMethod(inputMethodName string) uiauto.Action {
	return i.LeftClick(nodewith.Name("Remove " + inputMethodName).Role(role.Button))
}

// OpenInputMethodSetting opens the input method setting page in OS settings.
// The setting button is named as "Open settings page for " + im.Name.
// Japanese is the only exemption in "IME settings in the OS setting".
func (i *IMESettings) OpenInputMethodSetting(tconn *chrome.TestConn, im ime.InputMethod) uiauto.Action {
	return func(ctx context.Context) error {
		if im.Equal(ime.JapaneseWithUSKeyboard) || im.Equal(ime.Japanese) {
			return errors.Errorf("Open japanese settings in OS Settings is not supported %q", im)
		}

		imSettingButton := nodewith.Name("Open settings page for " + im.Name)
		imeSettingHeading := nodewith.Name(im.Name).Role(role.Heading).Ancestor(ossettings.WindowFinder)
		successCondition := uiauto.New(tconn).WithTimeout(5 * time.Second).WaitUntilExists(imeSettingHeading)
		return i.LeftClickUntil(imSettingButton, successCondition)(ctx)
	}
}

// ToggleShowInputOptionsInShelf clicks the 'Show input options in the shelf' toggle button to enable / disable the setting.
func (i *IMESettings) ToggleShowInputOptionsInShelf(cr *chrome.Chrome, tconn *chrome.TestConn, expected bool) uiauto.Action {
	return i.SetToggleOption(cr, tconn, string(ShowInputOptionsInShelf), expected)
}

// ToggleGlideTyping clicks the 'Enable glide typing' toggle button to enable / disable the setting.
func (i *IMESettings) ToggleGlideTyping(cr *chrome.Chrome, tconn *chrome.TestConn, expected bool) uiauto.Action {
	return i.SetToggleOption(cr, tconn, string(GlideTyping), expected)
}

// ToggleAutoCap clicks the 'Auto-capitalization' toggle button to enable / disable the setting.
func (i *IMESettings) ToggleAutoCap(cr *chrome.Chrome, tconn *chrome.TestConn, expected bool) uiauto.Action {
	return i.SetToggleOption(cr, tconn, string(AutoCapitalization), expected)
}
