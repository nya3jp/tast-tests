// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// InputModality describes the available input modalities.
type InputModality string

// Valid values for InputModality.
const (
	InputWithVK          InputModality = "Virtual Keyboard"
	InputWithVoice       InputModality = "Voice"
	InputWithHandWriting InputModality = "Handwriting"
	InputWithPK          InputModality = "Physical Keyboard"
)

// String for ACUITI detection
const acuitiString string = "acuiti"

// PKCandidatesFinder is the finder for candidates in the IME candidates window.
var PKCandidatesFinder = nodewith.Role(role.ImeCandidate).Onscreen()

// InputEval is a data structure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
}

// AppCompatTestCase is a data structure to define accent key test case for app compat test.
type AppCompatTestCase struct {
	TestName      string
	LanguageLabel string
	InputFunc     uiauto.Action
	InputMethod   ime.InputMethod
	TypeKeys      string
	ExpectedText  string
}

// WaitForFieldTextToBe returns an action checking whether the input field value equals given text.
// The text is case sensitive.
func WaitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return WaitForFieldTextToSatisfy(tconn, finder, expectedText, func(actualText string) bool {
		return expectedText == actualText
	})
}

// WaitForFieldTextToBeIgnoringCase returns an action checking whether the input field value equals given text.
// The text is case insensitive.
func WaitForFieldTextToBeIgnoringCase(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return WaitForFieldTextToSatisfy(tconn, finder, fmt.Sprintf("%s (ignoring case)", expectedText), func(actualText string) bool {
		return strings.ToLower(expectedText) == strings.ToLower(actualText)
	})
}

// WaitForFieldTextToSatisfy returns an action checking whether the input field value satisfies a predicate.
func WaitForFieldTextToSatisfy(tconn *chrome.TestConn, finder *nodewith.Finder, description string, predicate func(string) bool) uiauto.Action {
	ui := uiauto.New(tconn).WithInterval(time.Second)
	return uiauto.Combine("validate field text",
		// Sleep 200ms before validating text field.
		// Without sleep, it almost never pass the first time check due to the input delay.
		uiauto.Sleep(200*time.Millisecond),
		ui.RetrySilently(8, func(ctx context.Context) error {
			nodeInfo, err := ui.Info(ctx, finder)
			if err != nil {
				return err
			}

			if !predicate(nodeInfo.Value) {
				return errors.Errorf("failed to validate input value: got: %s; want: %s", nodeInfo.Value, description)
			}

			return nil
		}))
}

// WaitForFieldNotEmpty returns an action checking whether the input field value is not empty.
func WaitForFieldNotEmpty(tconn *chrome.TestConn, finder *nodewith.Finder) uiauto.Action {
	return WaitForFieldTextToSatisfy(tconn, finder, "not empty", func(actualText string) bool {
		return actualText != ""
	})
}

// GetNthCandidateText returns the candidate text in the specified position in the candidates window.
func GetNthCandidateText(ctx context.Context, tconn *chrome.TestConn, n int) (string, error) {
	ui := uiauto.New(tconn)

	candidate, err := ui.Info(ctx, PKCandidatesFinder.Nth(n))
	if err != nil {
		return "", err
	}

	return candidate.Name, nil
}

// RunSubtestsPerInputMethodAndMessage runs subtest that uses testName and inputdata on
// every combination of given input methods and messages.
func RunSubtestsPerInputMethodAndMessage(ctx context.Context, uc *useractions.UserContext, s *testing.State,
	inputMethods []ime.InputMethod, messages []data.Message, subtest func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State)) {
	for _, im := range inputMethods {
		// Setup input method.
		s.Logf("Set current input method to: %q", im)
		if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
			s.Fatalf("Failed to set input method to %q: %v: ", im, err)
		}
		uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

		for _, message := range messages {
			inputData, ok := message.GetInputData(im)
			if !ok {
				s.Fatalf("Test Data for input method %q does not exist", im)
			}
			testName := string(im.Name) + "-" + string(inputData.ExpectedText)

			s.Run(ctx, testName, subtest(testName, inputData))
		}
	}
}

// RunSubtestsPerInputMethodAndModalidy runs subtest that uses testName and inputdata on
// every combination of given input methods and messages.
func RunSubtestsPerInputMethodAndModalidy(ctx context.Context, uc *useractions.UserContext, s *testing.State,
	inputMethods []ime.InputMethod, messages map[InputModality]data.Message, subtest func(testName string, modality InputModality, inputData data.InputData) func(ctx context.Context, s *testing.State)) {
	for _, im := range inputMethods {
		// Setup input method.
		s.Logf("Set current input method to: %s", im)
		if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", im, err)
		}

		for modality, message := range messages {
			inputData, ok := message.GetInputData(im)
			if !ok {
				s.Fatalf("Test Data for input method %s does not exist", im)
			}
			testName := fmt.Sprintf("%s-%s-%s", im.Name, modality, inputData.ExpectedText)

			s.Run(ctx, testName, subtest(testName, modality, inputData))
		}
	}
}

// ExtractExternalFilesFromMap returns the file names contained in messages for
// selected input methods.
func ExtractExternalFilesFromMap(messages map[InputModality]data.Message, inputMethods []ime.InputMethod) []string {
	messageList := make([]data.Message, 0, len(messages))
	for _, message := range messages {
		messageList = append(messageList, message)
	}
	return data.ExtractExternalFiles(messageList, inputMethods)
}

// RunSubTest is designed to run an action as a subtest.
// It reserves 5s for general cleanup, dumping ui tree and screenshot on error.
func RunSubTest(ctx context.Context, s *testing.State, cr *chrome.Chrome, testName string, action uiauto.Action) {
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		if err := action(ctx); err != nil {
			s.Fatalf("Subtest %q failed: %v", testName, err)
		}
	})
}

// GetNthCandidateTextAndThen returns an action that performs two steps in sequence:
// 1) Get the specified candidate.
// 2) Pass the specified candidate into provided function and runs the returned action.
// This is used when an action depends on the text of a candidate.
func GetNthCandidateTextAndThen(tconn *chrome.TestConn, n int, fn func(text string) uiauto.Action) uiauto.Action {
	return func(ctx context.Context) error {
		text, err := GetNthCandidateText(ctx, tconn, n)
		if err != nil {
			return err
		}

		if err := fn(text)(ctx); err != nil {
			return err
		}

		return nil
	}
}

// IMESearchFlags generates searchFlags based on the list of input methods.
func IMESearchFlags(imes []ime.InputMethod) []*testing.StringPair {
	var searchFlags = []*testing.StringPair{}
	for _, ime := range imes {
		searchFlags = append(
			searchFlags,
			&testing.StringPair{
				Key:   "ime",
				Value: ime.Name,
			},
		)
	}
	return searchFlags
}

// VerifyTextShownFromScreenshot returns an action checking whether the text is shown on the screenshot or not
func VerifyTextShownFromScreenshot(tconn *chrome.TestConn, text string) uiauto.Action {
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)

	return uiauto.Combine("verify text shown on the screen",
		ud.WaitUntilExists(uidetection.Word(text, uidetection.MaxEditDistance(1), uidetection.DisableApproxMatch(true)).First()),
	)

}

// ClickEnterToStartNewLine return an action clicking the enter button on vk
func ClickEnterToStartNewLine(ctx context.Context) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		// s.Fatal("Failed to get keyboard: ", err)
		return errors.Wrap(err, "failed to get text location")
	}
	defer keyboard.Close()
	uiauto.Combine("Click enter button on vk to start new line",
		keyboard.AccelAction("Enter"),
	)(ctx)
	return nil
}

// TypingAccentKeyAccordingToLanguageOnVK return an action for typing an accent key with specific language on virtual keyboard
func TypingAccentKeyAccordingToLanguageOnVK(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, vkbCtx *vkb.VirtualKeyboardContext, languageLable, exptectedResult, keyName string) uiauto.Action {
	languageLabelFinder := vkb.NodeFinder.Name(languageLable).First()
	accentContainerFinder := nodewith.HasClass("accent-container")
	accentKeyFinder := nodewith.Ancestor(accentContainerFinder).Name(exptectedResult).Role(role.StaticText)
	keyFinder := vkb.KeyByNameIgnoringCase(keyName)

	return uiauto.Combine("input accent letter with virtual keyboard",
		vkbCtx.ShowVirtualKeyboard(),
		ui.WaitUntilExists(languageLabelFinder),
		ui.MouseMoveTo(keyFinder, 500*time.Millisecond),
		mouse.Press(tconn, mouse.LeftButton),
		// Popup accent window sometimes flash on showing, so using Retry instead of WaitUntilExist.
		ui.WithInterval(time.Second).RetrySilently(10, ui.WaitForLocation(accentContainerFinder)),
		ui.MouseMoveTo(accentKeyFinder, 500*time.Millisecond),
		mouse.Release(tconn, mouse.LeftButton),
		vkbCtx.HideVirtualKeyboard(),
		VerifyTextShownFromScreenshot(tconn, exptectedResult),
	)
}

// TypingLettersAccordingToLanguageOnVK return an action for typing letter with specific language on virtual keyboard
func TypingLettersAccordingToLanguageOnVK(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, vkbCtx *vkb.VirtualKeyboardContext, languageLable, exptectedResult string) uiauto.Action {
	languageLabelFinder := vkb.NodeFinder.Name(languageLable).First()
	return uiauto.Combine("input accent letter with virtual keyboard",
		vkbCtx.ShowVirtualKeyboard(),
		ui.WaitUntilExists(languageLabelFinder),
		vkbCtx.TapKeys(strings.Split(exptectedResult, "")),
		vkbCtx.HideVirtualKeyboard(),
		VerifyTextShownFromScreenshot(tconn, exptectedResult),
	)
}

// GlideTypingAccordingToLanguageOnVK return an action for typing letter with specific language on virtual keyboard
func GlideTypingAccordingToLanguageOnVK(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, vkbCtx *vkb.VirtualKeyboardContext, languageLable, exptectedResult string) uiauto.Action {
	languageLabelFinder := vkb.NodeFinder.Name(languageLable).First()

	validateAction := uiauto.Combine("hide virtual keyboard and do ACUITI verification",
		vkbCtx.HideVirtualKeyboard(),
		VerifyTextShownFromScreenshot(tconn, exptectedResult),
	)

	return uiauto.Combine("glide typing with virtual keyboard",
		vkbCtx.ShowVirtualKeyboard(),
		ui.WaitUntilExists(languageLabelFinder),
		vkbCtx.GlideTyping(strings.Split(exptectedResult, ""), validateAction),
	)
}

// InstallIME install the given input method
func InstallIME(ctx context.Context, uc *useractions.UserContext, inputMethod ime.InputMethod) error {
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		return errors.Wrap(err, "fail to set input method")
	}

	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)
	return nil
}

// TypingKeysAccordingToLanguageOnPK return an action for typing dead key with specific language on phyical keyboard
func TypingKeysAccordingToLanguageOnPK(ctx context.Context, tconn *chrome.TestConn, inputMethod ime.InputMethod, typingKeys, exptectedResult string) (uiauto.Action, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fail to get keyboard")
	}

	return uiauto.Combine("validate dead keys typing",
		kb.TypeAction(typingKeys),
		VerifyTextShownFromScreenshot(tconn, exptectedResult),
	), nil
}
