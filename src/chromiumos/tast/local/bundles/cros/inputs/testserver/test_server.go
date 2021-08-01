// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testserver contains methods to create a local web server for input tests and functions to set / get values of input fields.
package testserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
)

// InputField is the type of input field.
type InputField string

// Different type of input fields.
const (
	TextAreaInputField InputField = "textAreaInputField"
	TextInputField     InputField = "textInputField"
	SearchInputField   InputField = "searchInputField"
	PasswordInputField InputField = "passwordInputField"
	// PasswordTextField is not an editable input.
	// It is used for sync password value for visual testing.
	PasswordTextField              InputField = "passwordTextField"
	NumberInputField               InputField = "numberInputField"
	EmailInputField                InputField = "emailInputField"
	URLInputField                  InputField = "urlInputField"
	TelInputField                  InputField = "telInputField"
	DateInputField                 InputField = "dateInputField"
	MonthInputField                InputField = "monthInputField"
	WeekInputField                 InputField = "weekInputField"
	TimeInputField                 InputField = "timeInputField"
	DateTimeInputField             InputField = "dateTimeInputField"
	TextInputNumericField          InputField = "textInputNumericField"
	TextAreaNoCorrectionInputField InputField = "textArea disabled autocomplete, autocorrect, autocapitalize"

	// pageTitle is also the rootWebArea name in A11y to identify the scope of the page.
	pageTitle = "E14s test page"
)

// Inputs test page content.
const html = `<!DOCTYPE html>
<meta charset="utf-8">
<title>E14s test page</title>
<pre>No autocomplete</pre>
<textarea aria-label="textArea disabled autocomplete, autocorrect, autocapitalize" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" style="width: 100%"></textarea>
<br /><br />
<pre>&lt;<b>textarea</b> rows="7"&gt;&lt;/textarea&gt;</pre>
<textarea rows="7" aria-label="textAreaInputField" style="width: 100%"></textarea>
<br /><br />
<pre>&lt;input type="<b>text</b>"/&gt;</pre>
<input type="text" aria-label="textInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>search</b>"/&gt;</pre>
<input type="search" aria-label="searchInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>password</b>"/&gt;</pre>
<input id="passwordInput" type="password" aria-label="passwordInputField" style="width: 100%"
    oninput="document.getElementById('e14s-test-password-mirror').value = this.value;" />
<br />
<input id="e14s-test-password-mirror" aria-label="passwordTextField" type="text" readonly style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>number</b>"/&gt;</pre>
<input type="number" id="numberInput" aria-label="numberInputField" style="width: 100%" />
<br /><br />
<pre>No spellcheck (should have no autocorrect)</pre>
<textarea spellcheck="false" style="width:100%"></textarea>
<br /><br />
<pre><b>Dark Mode</b></pre>
<textarea rows="7" style="width: 100%;background-color:black;color:#fff"></textarea>
<br /><br />
<pre>&lt;input type="<b>email</b>"/&gt;</pre>
<input type="email" aria-label="emailInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>url</b>"/&gt;</pre>
<input type="url" aria-label="urlInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>tel</b>"/&gt;</pre>
<input type="tel" aria-label="telInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>date</b>"/&gt;</pre>
<input type="date" aria-label="dateInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>month</b>"/&gt;</pre>
<input type="month" aria-label="monthInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>week</b>"/&gt;</pre>
<input type="week" aria-label="weekInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>time</b>"/&gt;</pre>
<input type="time" aria-label="timeInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type="<b>datetime-local</b>"/&gt;</pre>
<input type="datetime-local" aria-label="dateTimeInputField" style="width: 100%" />
<br /><br />
<pre>&lt;input type=”text” inputmode=”numeric” pattern="[0-9]*"/&gt; (UK gov suggested numeric input for A11y)</pre>
<input type="text" inputmode="numeric" aria-label="textInputNumericField"/>`

// InputsTestServer is an unified server instance being used to manage web server and connection.
type InputsTestServer struct {
	server *httptest.Server
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	// Page connection. It is connected when loading the test page.
	// It is used for evaluate javascript.
	pc *chrome.Conn
	ui *uiauto.Context
}

// FieldInputEval encapsulates a function to input text into an input field, and its expected output.
type FieldInputEval struct {
	InputField   InputField
	InputFunc    uiauto.Action
	ExpectedText string
}

// pageRootFinder is the finder of root Node of the test page.
// All sub node should be located on the page.
var pageRootFinder = nodewith.Name(pageTitle).Role(role.RootWebArea)

// Finder returns the finder of the field by Name attribute.
func (inputField InputField) Finder() *nodewith.Finder {
	return nodewith.Ancestor(pageRootFinder).Name(string(inputField))
}

// Launch launches a local web server to serve inputs testing on different type of input fields.
// It then opens a Chrome browser window in normal mode to visit the test page.
func Launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*InputsTestServer, error) {
	return LaunchInMode(ctx, cr, tconn, false)
}

// LaunchInMode launches a local web server to serve inputs testing on different type of input fields.
// It can be either normal user mode or incognito mode.
func LaunchInMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, incognitoMode bool) (its *InputsTestServer, err error) {
	testing.ContextLog(ctx, "Start a local server to test inputs")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer func() {
		if err != nil {
			server.Close()
		}
	}()

	userMode := "normal"
	if incognitoMode {
		userMode = "incognito"
	}

	if err = apps.LaunchChromeByShortcut(tconn, incognitoMode)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to launch Chrome browser in %s mode", userMode)
	}

	var pc *chrome.Conn
	pc, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find new tab")
	}
	defer func() {
		if err != nil {
			pc.Close()
		}
	}()

	if err = pc.Navigate(ctx, server.URL); err != nil {
		return nil, errors.Wrapf(err, "failed to navigate to %q", server.URL)
	}

	if err = webutil.WaitForQuiescence(ctx, pc, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to load test page")
	}

	ui := uiauto.New(tconn)
	// Even document is ready, target is not yet in a11y tree.
	if err = ui.WaitUntilExists(pageRootFinder)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to render test page")
	}

	return &InputsTestServer{
		server: server,
		cr:     cr,
		tconn:  tconn,
		pc:     pc,
		ui:     ui,
	}, nil
}

// Close release the connection and stop the local web server.
func (its *InputsTestServer) Close() {
	its.pc.Close()
	its.server.Close()
}

// Clear returns an action clearing given input field by setting value to empty string via javascript.
func (its *InputsTestServer) Clear(inputField InputField) uiauto.Action {
	return func(ctx context.Context) error {
		return its.pc.Eval(ctx, fmt.Sprintf(`document.querySelector("*[aria-label='%s']").value=''`, inputField), nil)
	}
}

// WaitForFieldToBeActive returns an action waiting for certain input field to be the active element.
func (its *InputsTestServer) WaitForFieldToBeActive(inputField InputField) uiauto.Action {
	return func(ctx context.Context) error {
		return its.pc.WaitForExpr(ctx, fmt.Sprintf(`!!document.activeElement && document.querySelector("*[aria-label='%s']")===document.activeElement`, inputField))
	}
}

// ClickFieldAndWaitForActive returns an action clicking the input field and waiting for it to be active.
func (its *InputsTestServer) ClickFieldAndWaitForActive(inputField InputField) uiauto.Action {
	return uiauto.Combine(
		"click input field and wait for it to be active",
		its.ClickField(inputField),
		its.WaitForFieldToBeActive(inputField),
	)
}

// ClickField returns an action clicking the input field.
func (its *InputsTestServer) ClickField(inputField InputField) uiauto.Action {
	fieldFinder := inputField.Finder()
	return uiauto.Combine(
		"make input field visible on the screen and click it",
		its.ui.MakeVisible(fieldFinder),
		its.ui.LeftClick(fieldFinder),
	)
}

// ClickFieldUntilVKShown returns an action clicking the input field and waits for the virtual keyboard to show up.
func (its *InputsTestServer) ClickFieldUntilVKShown(inputField InputField) uiauto.Action {
	fieldFinder := inputField.Finder()
	return uiauto.Combine(
		"make input field visible on the screen and click it until virtual keyboard is shown",
		its.ui.MakeVisible(fieldFinder),
		// Use vkb.ClickUntilVKShown because it has retry internally.
		vkb.NewContext(its.cr, its.tconn).ClickUntilVKShown(fieldFinder),
	)
}

// ValidateInputOnField returns an action to test an input action on given input field.
// It clears field first and click to activate input.
// After input action, it checks whether the outcome equals to expected value.
func (its *InputsTestServer) ValidateInputOnField(inputField InputField, inputFunc uiauto.Action, expectedValue string) uiauto.Action {
	return uiauto.Combine("validate input function on field "+string(inputField),
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		inputFunc,
		util.WaitForFieldTextToBeIgnoringCase(its.tconn, inputField.Finder(), expectedValue),
	)
}

// ValidateVKInputOnField returns an action to test virtual keyboard input on given input field.
// After input action, it checks whether the outcome equals to expected value.
// This is deprecated, use ValidateInputFieldForMode instead.
// TODO(b/195208831): Deprecate the use of ValidateVKInputOnField.
func (its *InputsTestServer) ValidateVKInputOnField(vkbCtx *vkb.VirtualKeyboardContext, inputField InputField, inputData data.InputData) uiauto.Action {
	validateField := util.WaitForFieldTextToBeIgnoringCase(its.tconn, inputField.Finder(), inputData.ExpectedText)
	if inputField == PasswordInputField {
		// Password input is a special case. The value is presented with placeholder "•".
		// Using PasswordTextField field to verify the outcome.
		validateField = uiauto.Combine("validate passward field",
			util.WaitForFieldTextToBe(its.tconn, inputField.Finder(), strings.Repeat("•", len(inputData.CharacterKeySeq))),
			util.WaitForFieldTextToBeIgnoringCase(its.tconn, PasswordTextField.Finder(), inputData.ExpectedText),
		)
	}

	return uiauto.Combine("validate vk input function on field "+string(inputField),
		// Make sure virtual keyboard is not shown before action.
		vkbCtx.HideVirtualKeyboard(),
		its.Clear(inputField),
		its.ClickFieldUntilVKShown(inputField),
		vkbCtx.TapKeysIgnoringCase(inputData.CharacterKeySeq),
		func(ctx context.Context) error {
			if inputData.SubmitFromSuggestion {
				return vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx)
			}
			return nil
		},
		validateField,
	)
}

func (its *InputsTestServer) validateVKTypingInField(inputField InputField, inputData data.InputData) uiauto.Action {
	vkbCtx := vkb.NewContext(its.cr, its.tconn)
	return uiauto.Combine("validate vk input function on field "+string(inputField),
		its.cleanFieldAndTriggerVK(inputField),
		vkbCtx.TapKeysIgnoringCase(inputData.CharacterKeySeq),
		func(ctx context.Context) error {
			if inputData.SubmitFromSuggestion {
				return vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx)
			}
			return nil
		},
		its.validateResult(inputField, inputData.ExpectedText),
	)
}

func (its *InputsTestServer) validateVoiceInField(inputField InputField, inputData data.InputData, dataPath func(string) string) uiauto.Action {
	return func(ctx context.Context) error {
		// Setup CRAS Aloop for audio test.
		cleanup, err := voice.EnableAloop(ctx, its.tconn)
		if err != nil {
			return err
		}
		defer cleanup(ctx)

		vkbCtx := vkb.NewContext(its.cr, its.tconn)
		return uiauto.Combine("validate vk voice input function on field "+string(inputField),
			its.cleanFieldAndTriggerVK(inputField),
			vkbCtx.SwitchToVoiceInput(),
			func(ctx context.Context) error {
				return voice.AudioFromFile(ctx, dataPath(inputData.VoiceFile))
			},
			its.validateResult(inputField, inputData.ExpectedText),
		)(ctx)
	}
}

func (its *InputsTestServer) validateHandwritingInField(inputField InputField, inputData data.InputData, dataPath func(string) string) uiauto.Action {
	vkbCtx := vkb.NewContext(its.cr, its.tconn)
	return func(ctx context.Context) error {
		err := its.cleanFieldAndTriggerVK(inputField)(ctx)
		if err != nil {
			return err
		}

		hwCtx, err := vkbCtx.SwitchToHandwritingAndCloseInfoDialogue(ctx)
		if err != nil {
			return err
		}

		cleanupCtx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
		defer cancel()
		defer hwCtx.SwitchToKeyboard()(cleanupCtx)

		// Warm-up steps to check handwriting engine ready.
		checkEngineReady := uiauto.Combine("wait for handwriting engine to be ready",
			its.Clear(inputField),
			hwCtx.DrawStrokesFromFile(dataPath(inputData.HandwritingFile)),
			util.WaitForFieldNotEmpty(its.tconn, inputField.Finder()),
			hwCtx.ClearHandwritingCanvas(),
			its.Clear(inputField))

		return uiauto.Combine("handwriting input on virtual keyboard",
			hwCtx.WaitForHandwritingEngineReady(checkEngineReady),
			hwCtx.DrawStrokesFromFile(dataPath(inputData.HandwritingFile)),
			its.validateResult(inputField, inputData.ExpectedText),
		)(ctx)
	}
}

// ValidateInputFieldForMode returns an action to test input in the given field.
// After input action, it checks whether the outcome equals to expected value.
func (its *InputsTestServer) ValidateInputFieldForMode(inputField InputField, inputModality util.InputModality, inputData data.InputData, dataPath func(string) string) uiauto.Action {
	if !its.checkValidity(inputModality, inputField) {
		return func(ctx context.Context) error {
			return errors.Errorf("%s is not supported for %s", inputModality, inputField)
		}
	}
	// TODO(b/195083581): Enable ValidateInputFieldForMode for physical keyboard and emoji.
	switch inputModality {
	case util.InputWithVK:
		return its.validateVKTypingInField(inputField, inputData)
	case util.InputWithVoice:
		return its.validateVoiceInField(inputField, inputData, dataPath)
	case util.InputWithHandWriting:
		return its.validateHandwritingInField(inputField, inputData, dataPath)
	}

	return func(ctx context.Context) error {
		return errors.Errorf("input modality not supported: %q", inputModality)
	}
}

func (its *InputsTestServer) checkValidity(inputModality util.InputModality, inputField InputField) bool {
	if inputField == PasswordInputField {
		if inputModality == util.InputWithHandWriting || inputModality == util.InputWithVoice {
			return false
		}
	}
	return true
}

func (its *InputsTestServer) cleanFieldAndTriggerVK(inputField InputField) uiauto.Action {
	vkbCtx := vkb.NewContext(its.cr, its.tconn)
	return uiauto.Combine("clean and trigger VK on field "+string(inputField),
		vkbCtx.HideVirtualKeyboard(),
		its.Clear(inputField),
		its.ClickFieldUntilVKShown(inputField),
	)
}

func (its *InputsTestServer) validateResult(inputField InputField, expectedText string) uiauto.Action {
	validateField := util.WaitForFieldTextToBeIgnoringCase(its.tconn, inputField.Finder(), expectedText)
	if inputField == PasswordInputField {
		// Password input is a special case. The value is presented with placeholder "•".
		// Using PasswordTextField field to verify the outcome.
		validateField = uiauto.Combine("validate passward field",
			util.WaitForFieldTextToBe(its.tconn, inputField.Finder(), strings.Repeat("•", len(expectedText))),
			util.WaitForFieldTextToBeIgnoringCase(its.tconn, PasswordTextField.Finder(), expectedText),
		)
	}
	return validateField
}
