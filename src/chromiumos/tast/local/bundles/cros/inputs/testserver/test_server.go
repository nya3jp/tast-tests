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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

// Different type of input fields.
const (
	TextAreaInputField = "textAreaInputField"
	TextInputField     = "textInputField"
	SearchInputField   = "searchInputField"
	PasswordInputField = "passwordInputField"
	// PasswordTextField is not an editable input.
	// It is used for sync password value for visual testing.
	PasswordTextField              = "passwordTextField"
	NumberInputField               = "numberInputField"
	EmailInputField                = "emailInputField"
	URLInputField                  = "urlInputField"
	TelInputField                  = "telInputField"
	DateInputField                 = "dateInputField"
	MonthInputField                = "monthInputField"
	WeekInputField                 = "weekInputField"
	TimeInputField                 = "timeInputField"
	DateTimeInputField             = "dateTimeInputField"
	TextInputNumericField          = "textInputNumericField"
	TextAreaNoCorrectionInputField = "textArea disabled autocomplete, autocorrect, autocapitalize"

	// pageTitle is also the rootWebArea name in A11y to identify the scope of the page.
	pageTitle = "E14s test page"
)

// Inputs test page content.
const html = `<!DOCTYPE html>
<meta charset="utf-8">
<title>E14s test page</title>
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
<pre>No autocomplete</pre>
<textarea aria-label="textArea disabled autocomplete, autocorrect, autocapitalize" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" style="width: 100%"></textarea>
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

// pageRootFinder returns the finder of root Node of the test page.
// All sub node should be located on the page.
var pageRootFinder = nodewith.Name(pageTitle).Role(role.RootWebArea)

// Field returns the finder of the field by Name attribute.
func Field(fieldName string) *nodewith.Finder {
	return nodewith.Ancestor(pageRootFinder).Name(fieldName)
}

// Launch launches a local web server to serve inputs testing on different type of input fields.
func Launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*InputsTestServer, error) {
	testing.ContextLog(ctx, "Start a local server to test inputs")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))

	pc, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		server.Close()
		return nil, errors.Wrap(err, "failed to connect to inputs test server")
	}

	if err := pc.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		pc.Close()
		server.Close()
		return nil, errors.Wrap(err, "failed to load test page")
	}

	// Even document is ready, target is not yet in a11y tree.
	if err := uiauto.New(tconn).WaitUntilExists(pageRootFinder)(ctx); err != nil {
		pc.Close()
		server.Close()
		return nil, errors.Wrap(err, "failed to render test page")
	}

	return &InputsTestServer{
		server: server,
		cr:     cr,
		tconn:  tconn,
		pc:     pc,
		ui:     uiauto.New(tconn),
	}, nil
}

// Close release the connection and stop the local web server.
func (its *InputsTestServer) Close() {
	its.pc.Close()
	its.server.Close()
}

// Clear returns an action clearing given input field by setting value to empty string via javascript.
func (its *InputsTestServer) Clear(fieldName string) uiauto.Action {
	return func(ctx context.Context) error {
		return its.pc.Eval(ctx, fmt.Sprintf(`document.querySelector("*[aria-label='%s']").value=''`, fieldName), nil)
	}
}

// WaitForFieldToBeActive returns an action waiting for certain input field to be the active element.
func (its *InputsTestServer) WaitForFieldToBeActive(fieldName string) uiauto.Action {
	return func(ctx context.Context) error {
		return its.pc.WaitForExpr(ctx, fmt.Sprintf(`!!document.activeElement && document.querySelector("*[aria-label='%s']")===document.activeElement`, fieldName))
	}
}

// ClickFieldAndWaitForActive clicks the input field and waits for it to be active.
func (its *InputsTestServer) ClickFieldAndWaitForActive(fieldName string) uiauto.Action {
	return uiauto.Combine(
		"click input field and wait for it to be active",
		its.ClickField(fieldName),
		its.WaitForFieldToBeActive(fieldName),
	)
}

// ClickField returns an action clicking the input field.
func (its *InputsTestServer) ClickField(fieldName string) uiauto.Action {
	fieldFinder := Field(fieldName)
	return uiauto.Combine(
		"make input field visible on the screen and click it",
		its.ui.MakeVisible(fieldFinder),
		its.ui.LeftClick(fieldFinder),
	)
}

// ClickFieldUntilVKShown returns an action clicking the input field and waits for the virtual keyboard to show up.
func (its *InputsTestServer) ClickFieldUntilVKShown(fieldName string) uiauto.Action {
	fieldFinder := Field(fieldName)
	return uiauto.Combine(
		"make input field visible on the screen and click it until virtual keyboard is shown",
		its.ui.MakeVisible(fieldFinder),
		// Use vkb.ClickUntilVKShown because it has retry internally.
		vkb.NewContext(its.cr, its.tconn).ClickUntilVKShown(fieldFinder),
	)
}

// GetFieldValue returns current text in the input field.
func (its *InputsTestServer) GetFieldValue(ctx context.Context, fieldName string) (string, error) {
	nodeInfo, err := its.ui.Info(ctx, Field(fieldName))
	if err != nil {
		return "", errors.Wrap(err, "failed to get node info")
	}
	return nodeInfo.Value, nil
}

// WaitForFieldValueToBe returns an action repeatedly checking the input value until it matches the expectation.
func (its *InputsTestServer) WaitForFieldValueToBe(fieldName, expectedValue string) uiauto.Action {
	return its.ui.WithInterval(time.Second).Retry(5, func(ctx context.Context) error {
		if fieldValue, err := its.GetFieldValue(ctx, fieldName); err != nil {
			return err
		} else if fieldValue != expectedValue {
			return errors.Errorf("failed to validate input value: got: %s; want: %s", fieldValue, expectedValue)
		}
		return nil
	})
}

// ValidateInputOnField returns an action to test an input action on given input field.
// It clears field first and click to activate input.
// After input action, it checks whether the outcome equals to expected value.
func (its *InputsTestServer) ValidateInputOnField(fieldName string, inputFunc uiauto.Action, expectedValue string) uiauto.Action {
	return uiauto.Combine("validate input function on field "+fieldName,
		its.Clear(fieldName),
		its.ClickFieldAndWaitForActive(fieldName),
		inputFunc,
		its.WaitForFieldValueToBe(fieldName, expectedValue),
	)
}
