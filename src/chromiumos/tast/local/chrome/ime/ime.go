// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ime provides Go bindings of Chrome APIs to control IMEs.
package ime

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// AddAndSetInputMethod adds the IME identified by imeID and then sets it to the current input method.
// Note: this function will not do anything if the IME already exists.
func AddAndSetInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	if currentIME, err := CurrentInputMethod(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to get current input method")
	} else if currentIME == imeID {
		return nil
	}

	if err := AddInputMethod(ctx, tconn, imeID); err != nil {
		return errors.Wrapf(err, "failed to add input method %q", imeID)
	}
	return SetCurrentInputMethod(ctx, tconn, imeID)
}

// AddInputMethod adds the IME identified by imeID via
// chorme.languageSettingsPrivate.addInputMethod API.
func AddInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	if err := tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.addInputMethod`, imeID); err != nil {
		return errors.Wrapf(err, "failed to add input method %q", imeID)
	}
	if err := WaitForInputMethodInstalled(ctx, tconn, imeID, 20*time.Second); err != nil {
		return errors.Wrapf(err, "failed to wait for IME %q installed", imeID)
	}

	return nil
}

// RemoveInputMethod removes the IME identified by imeID via
// chorme.languageSettingsPrivate.removeInputMethod API.
func RemoveInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	if err := tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.removeInputMethod`, imeID); err != nil {
		return errors.Wrap(err, "failed to call chrome.languageSettingsPrivate.removeInputMethod")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		installedInputMethods, err := InstalledInputMethods(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get installed input methods"))
		}

		for _, inputMethod := range installedInputMethods {
			if inputMethod.ID == imeID {
				return errors.Wrapf(err, "failed to remove input method: %s", imeID)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// SetCurrentInputMethod sets the current input method to the IME identified imeID
// via chrome.inputMethodPrivate.setCurrentInputMethod API.
// It also waits 10s for new IME warming up.
func SetCurrentInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	return SetCurrentInputMethodAndWaitWarmUp(ctx, tconn, imeID, 10*time.Second)
}

// SetCurrentInputMethodAndWaitWarmUp sets the current input method to the IME identified imeID
// via chrome.inputMethodPrivate.setCurrentInputMethod API.
// It sleeps a certain time to wait for IME warming up.
func SetCurrentInputMethodAndWaitWarmUp(ctx context.Context, tconn *chrome.TestConn, imeID string, warmUpTime time.Duration) error {
	if err := tconn.Call(ctx, nil, `chrome.inputMethodPrivate.setCurrentInputMethod`, imeID); err != nil {
		return errors.Wrapf(err, "failed to set current input method to %q", imeID)
	}
	if err := WaitForInputMethodMatches(ctx, tconn, imeID, 20*time.Second); err != nil {
		return errors.Wrapf(err, "failed to wait for IME to be %q", imeID)
	}
	// Change IME takes time to install. There is no method to verify readiness of IME decoder.
	// This problem will be solved once decoder moved from Nacl to IME service.
	// TODO(b/157686038): Use API to identify completion of changing language
	return testing.Sleep(ctx, warmUpTime)
}

// CurrentInputMethod returns the ID of current IME obtained
// via chrome.inputMethodPrivate.getCurrentInputMethod API.
func CurrentInputMethod(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var imeID string
	err := tconn.Call(ctx, &imeID, `tast.promisify(chrome.inputMethodPrivate.getCurrentInputMethod)`)
	return imeID, err
}

// WaitForInputMethodMatches repeatedly checks until the current IME matches expectation.
func WaitForInputMethodMatches(ctx context.Context, tconn *chrome.TestConn, imeID string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		currentIME, err := CurrentInputMethod(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get current ime")
		}
		if currentIME != imeID {
			return errors.Errorf("got %q; want %q", currentIME, imeID)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// WaitForInputMethodInstalled repeatedly checks until a certain IME is installed.
func WaitForInputMethodInstalled(ctx context.Context, tconn *chrome.TestConn, imeID string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		inputMethods, err := InstalledInputMethods(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get installed input methods")
		}

		for _, inputMethod := range inputMethods {
			if strings.HasSuffix(inputMethod.ID, imeID) {
				return nil
			}
		}
		return errors.Wrapf(err, "%q is not found in installed input methods: %+v", imeID, inputMethods)
	}, &testing.PollOptions{Timeout: timeout})
}

// WaitForInputMethodRemoved repeatedly checks until a certain IME is uninstalled.
func WaitForInputMethodRemoved(ctx context.Context, tconn *chrome.TestConn, imeID string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		inputMethods, err := InstalledInputMethods(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get installed input methods")
		}

		for _, inputMethod := range inputMethods {
			if strings.HasSuffix(inputMethod.ID, imeID) {
				return errors.Wrapf(err, "%s is not removed", imeID)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// EnableLanguage enables the given language
// via chrome.languageSettingsPrivate.enableLanguage API.
func EnableLanguage(ctx context.Context, tconn *chrome.TestConn, lang string) error {
	return tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.enableLanguage`, lang)
}

// DisableLanguage disables the given language
// via chrome.languageSettingsPrivate.disableLanguage API.
func DisableLanguage(ctx context.Context, tconn *chrome.TestConn, lang string) error {
	return tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.disableLanguage`, lang)
}

// BindingInputMethod is the Go binding struct of
// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/common/extensions/api/language_settings_private.idl;l=55
// The struct only defines the necessary fields.
type BindingInputMethod struct {
	ID string `json:"id"`
}

// BindingInputMethodLists is the Go binding struct of
// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/common/extensions/api/language_settings_private.idl;l=75
// The struct only defines the necessary fields.
type BindingInputMethodLists struct {
	ThirdPartyExtensionIMEs []BindingInputMethod `json:"thirdPartyExtensionImes"`
}

// BrowserInputMethodLists returns supported InputMethodLists obtained
// via chrome.languageSettingsPrivate.getInputMethodLists API.
func BrowserInputMethodLists(ctx context.Context, tconn *chrome.TestConn) (*BindingInputMethodLists, error) {
	var imes BindingInputMethodLists
	if err := tconn.Call(ctx, &imes, `tast.promisify(chrome.languageSettingsPrivate.getInputMethodLists)`); err != nil {
		return nil, err
	}
	return &imes, nil
}

// InstalledInputMethods returns installed input methods
// via chrome.inputMethodPrivate.getInputMethods API.
func InstalledInputMethods(ctx context.Context, tconn *chrome.TestConn) ([]BindingInputMethod, error) {
	var inputMethods []BindingInputMethod
	if err := tconn.Call(ctx, &inputMethods, `tast.promisify(chrome.inputMethodPrivate.getInputMethods)`); err != nil {
		return nil, err
	}
	return inputMethods, nil
}

// Prefix returns the prefix of the default IME extension, depending on whether the build is Chrome (official build) or Chromium.
func Prefix(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	imes, err := InstalledInputMethods(ctx, tconn)
	if err != nil {
		return "", err
	}

	for _, ime := range imes {
		if strings.Contains(ime.ID, ChromeIMEPrefix) {
			return ChromeIMEPrefix, nil
		} else if strings.Contains(ime.ID, ChromiumIMEPrefix) {
			return ChromiumIMEPrefix, nil
		}
	}
	return "", errors.New("failed to detect the default IME extension")
}
