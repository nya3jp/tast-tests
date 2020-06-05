// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ime provides Go bindings of Chrome APIs to control IMEs.
package ime

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// AddInputMethod adds the IME identified by imeID via
// chorme.languageSettingsPrivate.addInputMethod API.
func AddInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	return tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.addInputMethod`, imeID)
}

// RemoveInputMethod removes the IME identified by imeID via
// chorme.languageSettingsPrivate.removeInputMethod API.
func RemoveInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	return tconn.Call(ctx, nil, `chrome.languageSettingsPrivate.removeInputMethod`, imeID)
}

// SetCurrentInputMethod sets the current input method to the IME identified imeID
// via chrome.inputMethodPrivate.setCurrentInputMethod API.
func SetCurrentInputMethod(ctx context.Context, tconn *chrome.TestConn, imeID string) error {
	return tconn.Call(ctx, nil, `chrome.inputMethodPrivate.setCurrentInputMethod`, imeID)
}

// GetCurrentInputMethod returns the ID of current IME obtained
// via chrome.inputMethodPrivate.getCurrentInputMethod API.
func GetCurrentInputMethod(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var imeID string
	err := tconn.Call(ctx, &imeID, `tast.promisify(chrome.inputMethodPrivate.getCurrentInputMethod)`)
	return imeID, err
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

// InputMethod is the Go binding struct of
// https://source.chromium.org/chromium/chromium/src/+/master:chrome/common/extensions/api/language_settings_private.idl;l=55
// The struct only defines the necessary fields.
type InputMethod struct {
	ID string `json:"id"`
}

// InputMethodLists is the Go binding struct of
// https://source.chromium.org/chromium/chromium/src/+/master:chrome/common/extensions/api/language_settings_private.idl;l=75
// The struct only defines the necessary fields.
type InputMethodLists struct {
	ThirdPartyExtensionIMEs []InputMethod `json:"thirdPartyExtensionImes"`
}

// GetInputMethodLists returns InputMethodLists obtained
// via chrome.languageSettingsPrivate.getInputMethodLists API.
func GetInputMethodLists(ctx context.Context, tconn *chrome.TestConn) (*InputMethodLists, error) {
	var imes InputMethodLists
	if err := tconn.Call(ctx, &imes, `tast.promisify(chrome.languageSettingsPrivate.getInputMethodLists)`); err != nil {
		return nil, err
	}
	return &imes, nil
}
