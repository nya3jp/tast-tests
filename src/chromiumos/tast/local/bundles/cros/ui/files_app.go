// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const filesAppID string = "hhaomjibdihmijegdhdafkllkbggdgoj"
const downloadPath string = "/home/chronos/user/Downloads/"
const textFile string = "text.txt"

// chrome.automation roles
const button string = "button"
const staticText string = "staticText"

// On screen elements
const myFiles string = "My files"
const downloads = "Downloads"
const newFolder = "New folder"

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesApp,
		Desc:         "Smoke test for Files app",
		Contacts:     []string{"bhansknecht@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func FilesApp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	//logs extra info if the test exits early in error
	logDebug := true
	defer func() {
		if logDebug {
			logDebugInfo(ctx, tconn, s)
		}
	}()

	fileLocation := filepath.Join(downloadPath, textFile)
	if err = ioutil.WriteFile(fileLocation, []byte("blahblah"), 0644); err != nil {
		s.Error("Failed to create file ", fileLocation, ": ", err)
	}
	defer os.Remove(downloadPath + textFile)

	if err = tconn.Exec(ctx, fmt.Sprintf("chrome.autotestPrivate.launchApp(%q, function(){})", filesAppID)); err != nil {
		s.Fatal("Failed to launch files app: ", err)
	}

	grabChromeAutomationRoot(ctx, tconn, s)
	clickElement(ctx, tconn, staticText, downloads, s)
	waitForElement(ctx, tconn, staticText, textFile, s, time.Minute)

	clickElement(ctx, tconn, button, "More…", s)
	waitForElement(ctx, tconn, staticText, newFolder, s, 10*time.Second)
	logDebug = false
}

func grabChromeAutomationRoot(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	if err := tconn.Exec(ctx, "var root; chrome.automation.getDesktop(r => root = r);"); err != nil {
		s.Fatal("Failed to grab chrome.automation root: ", err)
	}
}

func clickElement(ctx context.Context, tconn *chrome.Conn, role string, name string, s *testing.State) {
	waitForElement(ctx, tconn, role, name, s, 10*time.Second)
	clickQuery := fmt.Sprintf("element = root.find({attributes: {role: %q, name: %q}}); element.doDefault();", role, name)
	if err := tconn.Exec(ctx, clickQuery); err != nil {
		s.Fatalf("Failed to click element {role: %q, name: %q}: %s", role, name, err)
	}
}

func waitForElement(ctx context.Context, tconn *chrome.Conn, role string, name string, s *testing.State, timeout time.Duration) {
	findQuery := fmt.Sprintf("root.find({attributes: {role: %q, name: %q}})", role, name)
	if err := tconn.WaitForExprWithTimeoutFailOnErr(ctx, findQuery, timeout); err != nil {
		s.Fatalf("Failed to wait for element {role: %q, name: %q}: %s", role, name, err)
	}
}

func logDebugInfo(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	logRoot(ctx, tconn, s)
	logAllByRole(ctx, tconn, button, s)
	logAllByRole(ctx, tconn, staticText, s)
}

func logRoot(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	var data json.RawMessage
	if err := tconn.Eval(ctx, "root + ''", &data); err != nil {
		s.Error("Failed to log root node: ", err)
	}
	data[0] = '\n'
	data[len(data)-1] = '\n'
	s.Log(strings.Replace(strings.Replace(string(data[:]), "\\n", "\n", -1), "\\\"", "\"", -1))
}

func logAllByRole(ctx context.Context, tconn *chrome.Conn, role string, s *testing.State) {
	var elements []string
	findQuery := fmt.Sprintf(`var root;
		chrome.automation.getDesktop(r => root = r);
		root.findAll({attributes: {role: %q}}).map(node => node.name)`, role)
	if err := tconn.Eval(ctx, findQuery, &elements); err != nil {
		s.Errorf("Failed to grab debug info for {role: %s}: %s", role, err)
	}
	s.Logf("%s: %+q", role, elements)
}
