// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// ID is the Files App ID
	ID = "hhaomjibdihmijegdhdafkllkbggdgoj"

	// DownloadPath is the location of downloads for the test user
	DownloadPath = "/home/chronos/user/Downloads/"

	// RoleButton is the chrome.automation role for buttons
	RoleButton = "button"
	// RoleStaticText is the chrome.automation role for static text
	RoleStaticText = "staticText"
)

var roles = [...]string{RoleButton, RoleStaticText}

// LaunchFilesApp launches the files app and waits for it to be open.
// You should call CloseFilesApp at the end of your test
// If FilesApp fails to launch, an error is returned.
func LaunchFilesApp(ctx context.Context, tconn *chrome.Conn) error {
	if err := tconn.Exec(ctx, fmt.Sprintf("chrome.autotestPrivate.launchApp(%q, function(){})", ID)); err != nil {
		return err
	}
	// Initialize chrome automation
	if err := InitChromeAutomationRoot(ctx, tconn); err != nil {
		return err
	}
	// Wait for files app to be open
	return WaitForElement(ctx, tconn, RoleStaticText, "My files", time.Minute)
}

// CloseFilesApp closes the files app
func CloseFilesApp(ctx context.Context, tconn *chrome.Conn) {
	tconn.Exec(ctx, fmt.Sprintf("chrome.autotestPrivate.closeApp(%q, function(){})", ID))
}

// InitChromeAutomationRoot sets up chrome automation root for later calls.
// Uses variable root in javascript.
// If the initilizing root fails or takes too long, an error is returned.
func InitChromeAutomationRoot(ctx context.Context, tconn *chrome.Conn) error {
	if err := tconn.Exec(ctx, "var root; chrome.automation.getDesktop(r => root = r);"); err != nil {
		return err
	}
	//wait for root to be ready
	return tconn.WaitForExprWithTimeoutFailOnErr(ctx, "root", 10*time.Second)
}

// ClickElement clicks on the element with the specific role and name.
// You should make sure the element exists before trying to click it.
// If the javascript fails to execute, an error is returned.
func ClickElement(ctx context.Context, tconn *chrome.Conn, role string, name string) error {
	clickQuery := fmt.Sprintf("var element = root.find({attributes: {role: %q, name: %q}}); element.doDefault();", role, name)
	return tconn.Exec(ctx, clickQuery)
}

// WaitForElement waits for an element to exist.
// If the timeout is reached, an error is returned.
func WaitForElement(ctx context.Context, tconn *chrome.Conn, role string, name string, timeout time.Duration) error {
	findQuery := fmt.Sprintf("root.find({attributes: {role: %q, name: %q}})", role, name)
	return tconn.WaitForExprWithTimeoutFailOnErr(ctx, findQuery, timeout)
}

// SetNavigationOnElement set element as the navigation point for the next time the user presses Tab or Shift+Tab.
func SetNavigationOnElement(ctx context.Context, tconn *chrome.Conn, role string, name string) error {
	navQuery := fmt.Sprintf("var element = root.find({attributes: {role: %q, name: %q}}); element.setSequentialFocusNavigationStartingPoint();", role, name)
	return tconn.Exec(ctx, navQuery)
}

// LogDebugInfo logs all debug information.
func LogDebugInfo(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	LogRootDebugInfo(ctx, tconn, s)
	for _, r := range roles {
		LogRoleDebugInfo(ctx, tconn, r, s)
	}
}

// LogRootDebugInfo logs chrome.automation root.
func LogRootDebugInfo(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	var data json.RawMessage
	if err := tconn.Eval(ctx, "root + ''", &data); err != nil {
		s.Error("Failed to log root node: ", err)
		return
	}
	data[0] = '\n'
	data[len(data)-1] = '\n'
	s.Log(strings.Replace(strings.Replace(string(data[:]), "\\n", "\n", -1), "\\\"", "\"", -1))
}

// LogRoleDebugInfo logs all elements with a role.
func LogRoleDebugInfo(ctx context.Context, tconn *chrome.Conn, role string, s *testing.State) {
	var elements []string
	findQuery := fmt.Sprintf(`var root;
		chrome.automation.getDesktop(r => root = r);
		root.findAll({attributes: {role: %q}}).map(node => node.name)`, role)
	if err := tconn.Eval(ctx, findQuery, &elements); err != nil {
		s.Errorf("Failed to grab debug info for {role: %s}: %s", role, err)
		return
	}
	s.Logf("%s: %+q", role, elements)
}
