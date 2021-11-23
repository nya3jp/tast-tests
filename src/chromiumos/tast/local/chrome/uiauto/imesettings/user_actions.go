// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package imesettings

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
)

// EmojiSuggestionsOption represents the option name of Emoji suggestions toggle option.
const EmojiSuggestionsOption = "Emoji suggestions"

// AddInputMethodInOSSettings returns a user action adding certain input method in OS settings.
func AddInputMethodInOSSettings(uc *useractions.UserContext, kb *input.KeyboardEventWriter, im ime.InputMethod) *useractions.UserAction {
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

	return useractions.NewUserAction(
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
		})
}

// RemoveInputMethodInOSSettings returns a user action removing certain input method in OS settings.
func RemoveInputMethodInOSSettings(uc *useractions.UserContext, im ime.InputMethod) *useractions.UserAction {
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

	return useractions.NewUserAction(
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
		})
}

// SetEmojiSuggestions returns a user action to change 'Emoji suggestions' setting.
func SetEmojiSuggestions(uc *useractions.UserContext, isEnabled bool) *useractions.UserAction {
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

	return useractions.NewUserAction(actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagEmojiSuggestion,
			},
		})
}

// SetGlideTyping returns a user action to change 'Glide suggestions' setting.
func SetGlideTyping(uc *useractions.UserContext, im ime.InputMethod, isEnabled bool) *useractions.UserAction {
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

	return useractions.NewUserAction(
		actionName,
		action,
		uc,
		&useractions.UserActionCfg{
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
				useractions.ActionTagIMESettings,
				useractions.ActionTagGlideTyping,
			},
		})
}
