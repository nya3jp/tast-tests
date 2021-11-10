// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputactions

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// NewInputsUserContext returns a new user context structure designed for Essential Inputs.
func NewInputsUserContext(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, additionalAttributes map[string]string) (*useractions.UserContext, error) {
	attributes := make(map[string]string)
	// Merge additional attributes.
	if additionalAttributes != nil {
		attributes = additionalAttributes
	}

	// Save device mode to context attribute.
	isTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device mode")
	}
	attributes[useractions.AttributeDeviceMode] = useractions.DeviceModeClamshell
	if isTabletMode {
		attributes[useractions.AttributeDeviceMode] = useractions.DeviceModeTablet
	}

	attributes[useractions.AttributeKeyboardType] = useractions.KeyboardTypePhysicalKeyboard
	var a11yVKEnabled struct {
		Value bool `json:"value"`
	}
	if err := tconn.Call(ctx, &a11yVKEnabled, "tast.promisify(chrome.settingsPrivate.getPref)", "settings.a11y.virtual_keyboard"); err != nil {
		return nil, errors.Wrap(err, "failed to check whether a11y VK is enabled")
	}
	if a11yVKEnabled.Value {
		attributes[useractions.AttributeKeyboardType] = useractions.KeyboardTypeA11yVK
	} else if cr.VKEnabled() {
		attributes[useractions.AttributeKeyboardType] = useractions.KeyboardTypeTabletVK
	}

	// Save active input method to context attribute.
	activeInputMethod, err := ime.ActiveInputMethod(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active input method")
	}
	attributes[useractions.AttributeInputMethod] = activeInputMethod.Name

	return useractions.NewUserContext(s.TestName(), cr, tconn, s.OutDir(), attributes, []string{ActionTagEssentialInputs}), nil
}

// AddInputMethodInOSSettings returns an user action adding certain input method in OS settings.
func AddInputMethodInOSSettings(uc *useractions.UserContext, kb *input.KeyboardEventWriter, im ime.InputMethod) *useractions.UserAction {
	return useractions.NewUserAction(
		"Add input method in OS Settings",
		func(ctx context.Context) error {
			// Use the first 5 letters to search input method.
			// This will handle Unicode characters correctly.
			runes := []rune(im.Name)
			searchKeyword := string(runes[0:5])

			settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
			if err != nil {
				return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
			}
			if err := uiauto.Combine("add input method",
				settings.ClickAddInputMethodButton(),
				settings.SearchInputMethod(kb, searchKeyword, im.Name),
				settings.SelectInputMethod(im.Name),
				settings.ClickAddButtonToConfirm(),
				im.WaitUntilInstalled(uc.TestAPIConn()),
				settings.Close,
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to add input method")
			}
			return nil
		},
		uc,
		useractions.UserActionCfg{
			ActionAttributes: map[string]string{"AddedInputMethod": im.Name},
			ActionTags:       []string{ActionTagIMEManagement},
		})
}

// RemoveInputMethodInOSSettings returns an user action removing certain input method in OS settings.
func RemoveInputMethodInOSSettings(uc *useractions.UserContext, im ime.InputMethod) *useractions.UserAction {
	return useractions.NewUserAction(
		"Remove input method in OS Settings",
		func(ctx context.Context) error {
			settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
			if err != nil {
				return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
			}
			if err := uiauto.Combine("remove input method",
				settings.RemoveInputMethod(im.Name),
				im.WaitUntilRemoved(uc.TestAPIConn()),
				func(ctx context.Context) error {
					activeInputMethod, err := ime.ActiveInputMethod(ctx, uc.TestAPIConn())
					if err != nil {
						return errors.Wrap(err, "failed to get active input method")
					}
					uc.SetAttribute(useractions.AttributeInputMethod, activeInputMethod.Name)
					return nil
				},
				settings.Close,
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to remove input method")
			}
			return nil
		},
		uc,
		useractions.UserActionCfg{
			ActionAttributes: map[string]string{"RemovedInputMethod": im.Name},
			ActionTags:       []string{ActionTagIMEManagement},
		})
}

// SwitchToInputMethodWithShortcut returns an user action
// switching to certain input method with shortcut "Ctrl+Shift+Space".
func SwitchToInputMethodWithShortcut(uc *useractions.UserContext, kb *input.KeyboardEventWriter, im ime.InputMethod) *useractions.UserAction {
	return useractions.NewUserAction(
		"Switch to input method with Ctrl+Shift+Space",
		uiauto.Combine("switch to next input method and wait until it is activated",
			kb.AccelAction("Ctrl+Shift+Space"),
			im.WaitUntilActivated(uc.TestAPIConn()),
		),
		uc,
		useractions.UserActionCfg{
			ActionTags: []string{ActionTagIMEManagement},
			IfSuccessFunc: func(ctx context.Context) error {
				uc.SetAttribute(useractions.AttributeInputMethod, im.Name)
				return nil
			},
		})
}
