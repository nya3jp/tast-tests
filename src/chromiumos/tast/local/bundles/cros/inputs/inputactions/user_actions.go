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
	"chromiumos/tast/local/chrome/useractions"
)

type testingState interface {
	TestName() string
	OutDir() string
}

// NewInputsUserContext returns a new user context structure designed for Essential Inputs.
func NewInputsUserContext(ctx context.Context, s testingState, cr *chrome.Chrome, tconn *chrome.TestConn, additionalAttributes map[string]string) (*useractions.UserContext, error) {
	return NewInputsUserContextWithoutState(ctx, s.TestName(), s.OutDir(), cr, tconn, additionalAttributes)
}

// NewInputsUserContextWithoutState returns a new user context structure designed for Inputs fixtures.
func NewInputsUserContextWithoutState(ctx context.Context, testName, outDir string, cr *chrome.Chrome, tconn *chrome.TestConn, additionalAttributes map[string]string) (*useractions.UserContext, error) {
	attributes := make(map[string]string)
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

	// Save keyboard type to context attribute.
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

	// Save user login mode to context attribute.
	attributes[useractions.AttributeUserMode] = cr.LoginMode()

	// Save active input method to context attribute.
	activeInputMethod, err := ime.ActiveInputMethod(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active input method")
	}
	attributes[useractions.AttributeInputMethod] = activeInputMethod.Name

	return useractions.NewUserContext("", cr, tconn, outDir, attributes, []useractions.ActionTag{useractions.ActionTagEssentialInputs}), nil
}
