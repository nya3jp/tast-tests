// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package imesettings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
)

// EmojiSuggestionsOption represents the option name of Emoji suggestions toggle option.
const EmojiSuggestionsOption = "Emoji suggestions"

// AddInputMethodInOSSettings returns a user action adding certain input method in OS settings.
func AddInputMethodInOSSettings(uc *useractions.UserContext, kb *input.KeyboardEventWriter, im ime.InputMethod) uiauto.Action {
	action := func(ctx context.Context) error {
		// Use the first 5 letters to search input method.
		// This will handle Unicode characters correctly.
		runes := []rune(im.Name)
		searchKeyword := string(runes[0:5])

		settings, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
		}
		return uiauto.Combine("add input method",
			settings.ClickAddInputMethodButton(),
			settings.SearchInputMethod(kb, searchKeyword, im.Name),
			settings.SelectInputMethod(im.Name),
			settings.ClickAddButtonToConfirm(),
			im.WaitUntilInstalled(uc.TestAPIConn()),
			settings.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		"Add input method in OS Settings",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{"AddedInputMethod": im.Name},
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMEManagement,
				useractions.ActionTagAddIME,
			},
		},
	)
}

// RemoveInputMethodInOSSettings returns a user action removing certain input method in OS settings.
func RemoveInputMethodInOSSettings(uc *useractions.UserContext, im ime.InputMethod) uiauto.Action {
	action := func(ctx context.Context) error {
		settings, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
		}
		return uiauto.Combine("remove input method",
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
		)(ctx)
	}

	return uiauto.UserAction(
		"Remove input method in OS Settings",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{"RemovedInputMethod": im.Name},
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMEManagement,
				useractions.ActionTagRemoveIME,
			},
		},
	)
}

// SetEmojiSuggestions returns a user action to change 'Emoji suggestions' setting.
func SetEmojiSuggestions(uc *useractions.UserContext, isEnabled bool) uiauto.Action {
	actionName := "enable emoji suggestions in OS settings"
	if !isEnabled {
		actionName = "disable emoji suggestions in OS settings"
	}

	action := func(ctx context.Context) error {
		settings, err := LaunchAtSuggestionSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch OS settings and land at inputs setting page")
		}
		return uiauto.Combine("toggle setting and close page",
			settings.SetToggleOption(uc.Chrome(), EmojiSuggestionsOption, isEnabled),
			settings.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagEmojiSuggestion,
			},
		},
	)
}

// SetGlideTyping returns a user action to change 'Glide suggestions' setting.
func SetGlideTyping(uc *useractions.UserContext, im ime.InputMethod, isEnabled bool) uiauto.Action {
	actionName := "enable glide typing in IME setting"
	if !isEnabled {
		actionName = "disable glide typing in IME setting"
	}

	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}
		return uiauto.Combine("change glide typing setting",
			setting.OpenInputMethodSetting(uc.TestAPIConn(), im),
			setting.ToggleGlideTyping(uc.Chrome(), isEnabled),
			setting.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagGlideTyping,
			},
		},
	)
}

// AutoCorrectionLevel describes the auto correction level of an input method.
// The value exactly should exactly match the string displayed in IME setting page.
type AutoCorrectionLevel string

// Available auto correction levels.
const (
	AutoCorrectionOff         AutoCorrectionLevel = "Off"
	AutoCorrectionModest                          = "Modest"
	AutoCorrectionProgressive                     = "Progressive"
)

// SetVKAutoCorrection returns a user action to change 'On-screen keyboard Auto-correction' setting.
func SetVKAutoCorrection(uc *useractions.UserContext, im ime.InputMethod, acLevel AutoCorrectionLevel) uiauto.Action {
	return setAutoCorrection(uc, im, true, acLevel)
}

// SetPKAutoCorrection returns a user action to change 'Physical keyboard Auto-correction' setting.
func SetPKAutoCorrection(uc *useractions.UserContext, im ime.InputMethod, acLevel AutoCorrectionLevel) uiauto.Action {
	return setAutoCorrection(uc, im, false, acLevel)
}

func setAutoCorrection(uc *useractions.UserContext, im ime.InputMethod, isVK bool, acLevel AutoCorrectionLevel) uiauto.Action {
	actionName := fmt.Sprintf("Set PK auto-correction level to %q", acLevel)
	if isVK {
		actionName = fmt.Sprintf("Set VK auto-correction level to %q", acLevel)
	}

	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}
		return uiauto.Combine(actionName,
			setting.OpenInputMethodSetting(uc.TestAPIConn(), im),
			setting.setAutoCorrection(uc.Chrome(), isVK, string(acLevel)),
			// TODO(b/157686038) A better solution to identify decoder status.
			// Decoder works async in returning status to frontend IME and self loading.
			setting.Sleep(5*time.Second),
			setting.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagAutoCorrection,
			},
		},
	)
}

// SetVKAutoCapitalization returns a user action to change 'On-screen keyboard Auto-capitalization' setting.
func SetVKAutoCapitalization(uc *useractions.UserContext, im ime.InputMethod, isEnabled bool) uiauto.Action {
	actionName := "enable VK auto capitalization in IME setting"
	if !isEnabled {
		actionName = "disable VK auto capitalization in IME setting"
	}

	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}
		return uiauto.Combine(actionName,
			setting.OpenInputMethodSetting(uc.TestAPIConn(), im),
			setting.ToggleAutoCap(uc.Chrome(), isEnabled),
			// TODO(b/157686038) A better solution to identify decoder status.
			// Decoder works async in returning status to frontend IME and self loading.
			setting.Sleep(5*time.Second),
			setting.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagAutoCapitalization,
			},
		},
	)
}

// EnableInputOptionsInShelf returns a user action to change IME setting
// to show / hide IME options in shelf.
func EnableInputOptionsInShelf(uc *useractions.UserContext, shown bool) uiauto.Action {
	ui := uiauto.New(uc.TestAPIConn())
	imeMenuTrayButtonFinder := nodewith.Name("IME menu button").Role(role.Button)

	userActionName := "Show input options in the shelf"
	if !shown {
		userActionName = "Hide input options in the shelf"
	}

	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}

		return uiauto.Combine(strings.ToLower(userActionName),
			setting.ToggleShowInputOptionsInShelf(uc.Chrome(), shown),
			func(ctx context.Context) error {
				if shown {
					return ui.WaitUntilExists(imeMenuTrayButtonFinder)(ctx)
				}
				return ui.WaitUntilGone(imeMenuTrayButtonFinder)(ctx)
			},
			setting.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		userActionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagOSSettings,
				useractions.ActionTagIMEShelf,
			},
		},
	)
}

// SetKoreanKeyboardLayout returns a user action to change 'Korean keyboard layout' setting.
func SetKoreanKeyboardLayout(uc *useractions.UserContext, keyboardLayout string) uiauto.Action {
	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}

		return uiauto.Combine("test input method settings change",
			setting.OpenInputMethodSetting(uc.TestAPIConn(), ime.Korean),
			setting.ChangeKoreanKeyboardLayout(uc.Chrome(), keyboardLayout),
			setting.Close,
		)(ctx)
	}

	return uiauto.UserAction(
		"Change Korean keyboard layout setting",
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
			},
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf("Change layout to %q", keyboardLayout),
			},
			Callback: func(ctx context.Context, actionError error) error {
				if actionError == nil {
					uc.SetAttribute(useractions.AttributeKeyboardLayout, keyboardLayout)
				}
				return nil
			},
		},
	)
}
