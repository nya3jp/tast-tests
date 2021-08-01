// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// InputModality describes the available input modalities.
type InputModality string

// Valid values for InputModality.
const (
	InputWithVK          InputModality = "Virtual Keyboard"
	InputWithVoice       InputModality = "Voice"
	InputWithHandWriting InputModality = "Handwriting"
)

// PKCandidatesFinder is the finder for candidates in the IME candidates window.
var PKCandidatesFinder = nodewith.Role(role.ImeCandidate)

// InputEval is a data structure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
}

// WaitForFieldTextToBe returns an action checking whether the input field value equals given text.
// The text is case sensitive.
func WaitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return waitForFieldTextFunc(tconn, finder, expectedText, false)
}

// WaitForFieldTextToBeIgnoringCase returns an action checking whether the input field value equals given text.
// The text is case insensitive.
func WaitForFieldTextToBeIgnoringCase(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return waitForFieldTextFunc(tconn, finder, expectedText, true)
}

// waitForFieldTextFunc returns an action checking whether the input field value equals given text.
// The text can either be case sensitive or not.
func waitForFieldTextFunc(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string, ignoreCase bool) uiauto.Action {
	ui := uiauto.New(tconn).WithInterval(time.Second)
	return uiauto.Combine("validate field text",
		// Sleep 200ms before validating text field.
		// Without sleep, it almost never pass the first time check due to the input delay.
		ui.Sleep(200*time.Millisecond),
		ui.Retry(5, func(ctx context.Context) error {
			nodeInfo, err := ui.Info(ctx, finder)
			if err != nil {
				return err
			} else if !ignoreCase && nodeInfo.Value != expectedText {
				return errors.Errorf("failed to validate input value: got: %s; want: %s", nodeInfo.Value, expectedText)
			} else if ignoreCase && strings.ToLower(nodeInfo.Value) != strings.ToLower(expectedText) {
				return errors.Errorf("failed to validate input value ignoring case: got: %s; want: %s", nodeInfo.Value, expectedText)
			}
			return nil
		}))
}

// WaitForFieldNotEmpty returns an action checking whether the input field value is not empty.
func WaitForFieldNotEmpty(tconn *chrome.TestConn, finder *nodewith.Finder) uiauto.Action {
	ui := uiauto.New(tconn).WithInterval(time.Second)
	return uiauto.Combine("validate field text",
		ui.Retry(5, func(ctx context.Context) error {
			nodeInfo, err := ui.Info(ctx, finder)
			if err != nil {
				return err
			} else if nodeInfo.Value == "" {
				return errors.New("input field is empty")
			}
			return nil
		}))
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
func RunSubtestsPerInputMethodAndMessage(ctx context.Context, tconn *chrome.TestConn, s *testing.State,
	inputMethods []ime.InputMethod, messages []data.Message, subtest func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State)) {
	for _, im := range inputMethods {
		// Setup input method.
		s.Logf("Set current input method to: %q", im)
		if err := im.InstallAndActivate(tconn)(ctx); err != nil {
			s.Fatalf("Failed to set input method to %q: %v: ", im, err)
		}

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
