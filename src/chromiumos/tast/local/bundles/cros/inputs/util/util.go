// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
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

// PKCandidatesFinder is the finder for candidates in the IME candidates window.
var PKCandidatesFinder = nodewith.Role(role.ImeCandidate).Onscreen()

// InputEval is a data structure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
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
		if err := im.InstallAndActivate(uc.TestAPIConn())(ctx); err != nil {
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
func RunSubtestsPerInputMethodAndModalidy(ctx context.Context, tconn *chrome.TestConn, s *testing.State,
	inputMethods []ime.InputMethod, messages map[InputModality]data.Message, subtest func(testName string, modality InputModality, inputData data.InputData) func(ctx context.Context, s *testing.State)) {
	for _, im := range inputMethods {
		// Setup input method.
		s.Logf("Set current input method to: %s", im)
		if err := im.InstallAndActivate(tconn)(ctx); err != nil {
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
