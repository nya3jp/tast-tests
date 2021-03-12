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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/vkb"
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
	conn   *chrome.Conn
}

// Launch launches a local web server to serve inputs testing on different type of input fields.
func Launch(ctx context.Context, cr *chrome.Chrome) (*InputsTestServer, error) {
	testing.ContextLog(ctx, "Start a local server to test inputs")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))

	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		server.Close()
		return nil, errors.Wrap(err, "failed to connect to inputs test server")
	}

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		conn.Close()
		server.Close()
		return nil, errors.Wrap(err, "failed to load test page")
	}

	return &InputsTestServer{
		server: server,
		conn:   conn,
	}, nil
}

// Close release the connection and stop the local web server.
func (its *InputsTestServer) Close() {
	its.conn.Close()
	its.server.Close()
}

// Clear returns an action clearing given input field by setting value to empty string via javascript.
func (its *InputsTestServer) Clear(inputField InputField) uiauto.Action {
	return func(ctx context.Context) error {
		return its.conn.Eval(ctx, fmt.Sprintf(`document.querySelector("*[aria-label='%s']").value=''`, string(inputField)), nil)
	}
}

// WaitForFieldToBeActive waits for certain input field to be the active element.
func (its *InputsTestServer) WaitForFieldToBeActive(ctx context.Context, inputField InputField) error {
	return its.conn.WaitForExpr(ctx, fmt.Sprintf(`!!document.activeElement && document.querySelector("*[aria-label='%s']")===document.activeElement`, string(inputField)))
}

// ClickFieldAndWaitForActive clicks the input field and waits for it to be active.
func (its *InputsTestServer) ClickFieldAndWaitForActive(ctx context.Context, tconn *chrome.TestConn, inputField InputField) error {
	if err := inputField.Click(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to click %s", inputField)
	}
	return its.WaitForFieldToBeActive(ctx, inputField)
}

// Click clicks the input field.
func (inputField InputField) Click(ctx context.Context, tconn *chrome.TestConn) error {
	actionFunc := func(node *ui.Node) error {
		if err := node.MakeVisible(ctx); err != nil {
			return errors.Wrapf(err, "failed to make the %s input field visible", string(inputField))
		}
		if err := node.WaitLocationStable(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
			return errors.Wrapf(err, "failed to wait for %s input field location stable", inputField)
		}
		return node.LeftClick(ctx)
	}
	return inputField.action(ctx, tconn, actionFunc)
}

// ClickUntilVKShown clicks the input field and waits for the virtual keyboard to show up.
func (inputField InputField) ClickUntilVKShown(ctx context.Context, tconn *chrome.TestConn) error {
	actionFunc := func(node *ui.Node) error {
		if err := node.MakeVisible(ctx); err != nil {
			return errors.Wrapf(err, "failed to make the %s input field visible", string(inputField))
		}
		if err := node.WaitLocationStable(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
			return errors.Wrapf(err, "failed to wait for %s input field location stable", inputField)
		}
		return vkb.ClickUntilVKShown(ctx, tconn, node)
	}
	return inputField.action(ctx, tconn, actionFunc)
}

// GetValue returns current text in the input field.
func (inputField InputField) GetValue(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var out string

	actionFunc := func(node *ui.Node) error {
		if err := node.Update(ctx); err != nil {
			return errors.Wrap(err, "failed to update node")
		}
		out = node.Value
		return nil
	}

	err := inputField.action(ctx, tconn, actionFunc)

	return out, err
}

// WaitForValueToBe repeatedly checks the input value until it matches the expectation.
func (inputField InputField) WaitForValueToBe(ctx context.Context, tconn *chrome.TestConn, expectedValue string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		currentValue, err := inputField.GetValue(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get input value"))
		}
		if currentValue != expectedValue {
			return errors.Errorf("failed to validate input value: got: %s; want: %s", currentValue, expectedValue)
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second})
}

// GetNode returns the a11y node of the input field.
func (inputField InputField) GetNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	pageRoot, err := pageRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find page root node")
	}
	defer pageRoot.Release(ctx)

	node, err := pageRoot.DescendantWithTimeout(ctx, ui.FindParams{Name: string(inputField)}, 10*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find input field: %s", string(inputField))
	}
	return node, nil
}

func (inputField InputField) action(ctx context.Context, tconn *chrome.TestConn, f func(node *ui.Node) error) error {
	node, err := inputField.GetNode(ctx, tconn)
	if err != nil {
		return errors.Wrapf(err, "failed to find input field: %s", string(inputField))
	}
	defer node.Release(ctx)

	return f(node)
}

// pageRootNode returns the root Node of the inputs test page. All sub node should be located under the page.
func pageRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRootWebArea, Name: pageTitle}, 10*time.Second)
}
