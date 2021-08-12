// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
)

// TODO(b/192819861): Define new input method struct and migrate existing use of InputMethodCode.
// This page is a partial implementation of b/192819861.
// This structure might need to be refined.

// ID is the unique identifier of an input method
// from http://osscs/chromium/chromium/src/+/main:chrome/browser/resources/chromeos/input_method/google_xkb_manifest.json.
// Name is likely to change due to CrOS Cross-border improvement.

// InputMethod represents an input method.
type InputMethod struct {
	Name                string   // The displayed name of the IME in OS Settings.
	ID                  string   // The code / id of the IME, e.g. "xkb:us::eng"
	HandwritingLanguage Language // The language for handwriting.
	VoiceLanguage       Language // The language for voice dictation.
}

// DefaultInputMethod is the default input method enabled for new users.
var DefaultInputMethod = EnglishUS

// EnglishUS represents the input method of English (US).
var EnglishUS = InputMethod{
	Name:                "English (US)",
	ID:                  "xkb:us::eng",
	HandwritingLanguage: LanguageEn,
	VoiceLanguage:       LanguageEn,
}

// EnglishUSWithInternationalKeyboard represents the input method of English (US) with International keyboard.
var EnglishUSWithInternationalKeyboard = InputMethod{
	Name:                "English (US) with International keyboard",
	ID:                  "xkb:us:intl:eng",
	HandwritingLanguage: LanguageEn,
	VoiceLanguage:       LanguageEn,
}

// EnglishUK represents the input method of English (UK).
var EnglishUK = InputMethod{
	Name:                "English (UK)",
	ID:                  "xkb:gb:extd:eng",
	HandwritingLanguage: LanguageEn,
	VoiceLanguage:       LanguageEn,
}

// SpanishSpain represents the input method of Spanish (Spain).
var SpanishSpain = InputMethod{
	Name:                "Spanish (Spain)",
	ID:                  "xkb:es::spa",
	HandwritingLanguage: LanguageEs,
	VoiceLanguage:       LanguageEs,
}

// Swedish represents the input method of Swedish.
var Swedish = InputMethod{
	Name:                "Swedish",
	ID:                  "xkb:se::swe",
	HandwritingLanguage: LanguageSv,
	VoiceLanguage:       LanguageSv,
}

// AlphanumericWithJapaneseKeyboard represents the input method of Alphanumeric with Japanese keyboard.
var AlphanumericWithJapaneseKeyboard = InputMethod{
	Name:                "Alphanumeric with Japanese keyboard",
	ID:                  "xkb:jp::jpn",
	HandwritingLanguage: LanguageJa,
	VoiceLanguage:       LanguageJa,
}

// EnglishCanada represents the input method of English (Canada).
var EnglishCanada = InputMethod{
	Name:                "English (Canada)",
	ID:                  "xkb:ca:eng:eng",
	HandwritingLanguage: LanguageEn,
	VoiceLanguage:       LanguageEn,
}

// Japanese represents the input method of Japanese.
var Japanese = InputMethod{
	Name:                "Japanese",
	ID:                  "nacl_mozc_jp",
	HandwritingLanguage: LanguageJa,
	VoiceLanguage:       LanguageJa,
}

// FrenchFrance represents the input method of French (France).
var FrenchFrance = InputMethod{
	Name:                "French (France)",
	ID:                  "xkb:fr::fra",
	HandwritingLanguage: LanguageFr,
	VoiceLanguage:       LanguageFr,
}

// JapaneseWithUSKeyboard represents the input method of Japanese with US keyboard.
var JapaneseWithUSKeyboard = InputMethod{
	Name:                "Japanese with US keyboard",
	ID:                  "nacl_mozc_us",
	HandwritingLanguage: LanguageJa,
	VoiceLanguage:       LanguageJa,
}

// ChinesePinyin represents the input method of Chinese Pinyin.
var ChinesePinyin = InputMethod{
	Name:                "Chinese Pinyin",
	ID:                  "zh-t-i0-pinyin",
	HandwritingLanguage: LanguageZhHans,
	VoiceLanguage:       LanguageZhHans,
}

// Cantonese represents the input method of Chinese Cantonese.
var Cantonese = InputMethod{
	Name:                "Cantonese",
	ID:                  "yue-hant-t-i0-und",
	HandwritingLanguage: LanguageZhHant,
	VoiceLanguage:       LanguageYueHant,
}

// ChineseCangjie represents the input method of Chinese Cangjie.
var ChineseCangjie = InputMethod{
	Name:                "Chinese Cangjie",
	ID:                  "zh-hant-t-i0-cangjie-1987",
	HandwritingLanguage: LanguageZhHant,
	VoiceLanguage:       LanguageZhHant,
}

// Korean represents the input method of Korean.
var Korean = InputMethod{
	Name:                "Korean",
	ID:                  "ko-t-i0-und",
	HandwritingLanguage: LanguageKo,
	VoiceLanguage:       LanguageKo,
}

// Arabic represents the input method of Arabic.
var Arabic = InputMethod{
	Name:                "Arabic",
	ID:                  "vkd_ar",
	HandwritingLanguage: LanguageAr,
	VoiceLanguage:       LanguageAr,
}

// inputMethods represents in-use (available) IMEs in ChromeOS.
// Only listed input methods are promised to be available.
var inputMethods = []InputMethod{
	EnglishUS,
	EnglishUSWithInternationalKeyboard,
	EnglishUK,
	SpanishSpain,
	Swedish,
	AlphanumericWithJapaneseKeyboard,
	EnglishCanada,
	Japanese,
	FrenchFrance,
	JapaneseWithUSKeyboard,
	ChinesePinyin,
	Cantonese,
	ChineseCangjie,
	Korean,
	Arabic,
}

// ActiveInputMethod returns the active input method via Chrome API.
func ActiveInputMethod(ctx context.Context, tconn *chrome.TestConn) (*InputMethod, error) {
	fullyQualifiedIMEID, err := CurrentInputMethod(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return FindInputMethodByFullyQualifiedIMEID(ctx, tconn, fullyQualifiedIMEID)
}

// FindInputMethodByName finds the input method by displayed name.
func FindInputMethodByName(name string) (*InputMethod, error) {
	for _, im := range inputMethods {
		if im.Name == name {
			return &im, nil
		}
	}
	return nil, errors.Errorf("failed to find input method by name %q", name)
}

// FindInputMethodByID finds the input method by ime id.
func FindInputMethodByID(id string) (*InputMethod, error) {
	for _, im := range inputMethods {
		if im.ID == id {
			return &im, nil
		}
	}
	return nil, errors.Errorf("failed to find input method by IME id %q", id)
}

// FindInputMethodByFullyQualifiedIMEID finds the input method by fully qualified IME ID,
// e.g. _comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:us::eng.
func FindInputMethodByFullyQualifiedIMEID(ctx context.Context, tconn *chrome.TestConn, fullyQualifiedIMEID string) (*InputMethod, error) {
	imePrefix, err := Prefix(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get IME prefix")
	}
	for _, im := range inputMethods {
		if imePrefix+im.ID == fullyQualifiedIMEID {
			return &im, nil
		}
	}
	return nil, errors.Errorf("failed to find input method by IME Code %q", fullyQualifiedIMEID)
}

// FullyQualifiedIMEID returns the fully qualified IME id constructed by IMEPrefix + IME ID.
// In Chrome, the fully qualified IME id is Chrome IME prefix + id: e.g. _comp_ime_jkghodnilhceideoidjikpgommlajknkxkb:us::eng
// In Chromium, the fully qualified IME id is Chromium IME prefix + id: e.g. _comp_ime_fgoepimhcoialccpbmpnnblemnepkkaoxkb:us::eng
func (im InputMethod) FullyQualifiedIMEID(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	imePrefix, err := Prefix(ctx, tconn)
	if err != nil {
		return "", errors.Wrap(err, "failed to get IME prefix")
	}
	return imePrefix + im.ID, nil
}

// Equal compares two input methods by id and returns true if they equal.
func (im InputMethod) Equal(imb InputMethod) bool {
	return im.ID == imb.ID
}

// String returns the key representative string content of the input method.
func (im InputMethod) String() string {
	return fmt.Sprintf("ID: %s; Name: %s", im.ID, im.Name)
}

// Install installs the input method via Chrome API.
// It does nothing if the IME is already installed.
func (im InputMethod) Install(tconn *chrome.TestConn) action.Action {
	f := func(ctx context.Context, fullyQualifiedIMEID string) error {
		fullyQualifiedIMEs, err := InstalledInputMethods(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get installed input methods")
		}

		for _, installedIME := range fullyQualifiedIMEs {
			if installedIME.ID == fullyQualifiedIMEID {
				return nil
			}
		}
		return AddInputMethod(ctx, tconn, fullyQualifiedIMEID)
	}
	return im.actionWithFullyQualifiedID(tconn, f)
}

// WaitUntilInstalled waits for the input method to be installed.
func (im InputMethod) WaitUntilInstalled(tconn *chrome.TestConn) action.Action {
	f := func(ctx context.Context, fullyQualifiedIMEID string) error {
		return WaitForInputMethodInstalled(ctx, tconn, fullyQualifiedIMEID, 20*time.Second)
	}
	return im.actionWithFullyQualifiedID(tconn, f)
}

// WaitUntilRemoved waits for the input method to be removed.
func (im InputMethod) WaitUntilRemoved(tconn *chrome.TestConn) action.Action {
	f := func(ctx context.Context, fullyQualifiedIMEID string) error {
		return WaitForInputMethodRemoved(ctx, tconn, fullyQualifiedIMEID, 20*time.Second)
	}
	return im.actionWithFullyQualifiedID(tconn, f)
}

// Activate sets the input method to use via Chrome API.
// It does nothing if the IME is already in use.
func (im InputMethod) Activate(tconn *chrome.TestConn) action.Action {
	f := func(ctx context.Context, fullyQualifiedIMEID string) error {
		activeIME, err := ActiveInputMethod(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get active input method")
		}

		if activeIME.Equal(im) {
			return nil
		}

		// Use 10s as warming up time by default.
		imWarmingUpTime := 10 * time.Second

		// SW, FR, SP takes longer time.
		switch im {
		case Swedish, FrenchFrance, SpanishSpain:
			imWarmingUpTime = 15 * time.Second
		}

		return SetCurrentInputMethodAndWaitWarmUp(ctx, tconn, fullyQualifiedIMEID, imWarmingUpTime)
	}
	return im.actionWithFullyQualifiedID(tconn, f)
}

// InstallAndActivate installs the input method and set it to active via Chrome API.
func (im InputMethod) InstallAndActivate(tconn *chrome.TestConn) action.Action {
	return uiauto.Combine(fmt.Sprintf("install and activate input method: %q", im),
		im.Install(tconn),
		im.Activate(tconn),
	)
}

// Remove uninstalls the input method via Chrome API.
func (im InputMethod) Remove(tconn *chrome.TestConn) action.Action {
	f := func(ctx context.Context, fullyQualifiedIMEID string) error {
		return RemoveInputMethod(ctx, tconn, fullyQualifiedIMEID)
	}
	return im.actionWithFullyQualifiedID(tconn, f)
}

func (im InputMethod) actionWithFullyQualifiedID(tconn *chrome.TestConn, f func(ctx context.Context, fullyQualifiedIMEID string) error) action.Action {
	return func(ctx context.Context) error {
		fullyQualifiedIMEID, err := im.FullyQualifiedIMEID(ctx, tconn)
		if err != nil {
			return errors.Wrapf(err, "failed to get fully qualified IME ID of %q", im)
		}
		return f(ctx, fullyQualifiedIMEID)
	}
}

// SetSettings changes the IME setting via chrome api.
// `chrome.inputMethodPrivate.setSettings(
//     "xkb:us::eng", { "physicalKeyboardAutoCorrectionLevel": 1})`,
// Note: Settings change won't take effect until the next input session.
// e.g. focus on a text field, or change input method.
// Live setting change is not supported because it never happens in a real user environment.
func (im InputMethod) SetSettings(tconn *chrome.TestConn, settings map[string]interface{}) action.Action {
	return func(ctx context.Context) error {
		settingJSON, err := json.Marshal(settings)
		if err != nil {
			return errors.Wrapf(err, "failed to read settings: %+v", settings)
		}

		var settingsAPICall = fmt.Sprintf(
			`chrome.inputMethodPrivate.setSettings(
					 %q, %s)`,
			im.ID, settingJSON)

		return tconn.Eval(ctx, settingsAPICall, nil)
	}
}

// ResetSettings empties IME settings to reset.
func (im InputMethod) ResetSettings(tconn *chrome.TestConn) action.Action {
	return im.SetSettings(tconn, map[string]interface{}{})
}

// SetPKAutoCorrection whether enables or disables the physical keyboard auto correction.
func (im InputMethod) SetPKAutoCorrection(tconn *chrome.TestConn, acLevel AutoCorrectionLevel) action.Action {
	settings := map[string]interface{}{"physicalKeyboardAutoCorrectionLevel": acLevel}
	return im.SetSettings(tconn, settings)
}

// SetVKAutoCorrection whether enables or disables the physical keyboard auto correction.
func (im InputMethod) SetVKAutoCorrection(tconn *chrome.TestConn, acLevel AutoCorrectionLevel) action.Action {
	settings := map[string]interface{}{"virtualKeyboardAutoCorrectionLevel": acLevel}
	return im.SetSettings(tconn, settings)
}

// SetVKEnableCapitalization whether enables or disables auto capitalization.
func (im InputMethod) SetVKEnableCapitalization(tconn *chrome.TestConn, isEnabled bool) action.Action {
	settings := map[string]interface{}{"virtualKeyboardEnableCapitalization": isEnabled}
	return im.SetSettings(tconn, settings)
}

// AutoCorrectionLevel describes the auto correction level of an input method.
type AutoCorrectionLevel int

// Available auto correction levels.
const (
	AutoCorrectionOff AutoCorrectionLevel = iota
	AutoCorrectionModest
	AutoCorrectionProgressive
)
