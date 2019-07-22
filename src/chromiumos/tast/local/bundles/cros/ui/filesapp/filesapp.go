// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const filesAppID = "hhaomjibdihmijegdhdafkllkbggdgoj"

const downloadPath = "/home/chronos/user/Downloads/"
const previewImageFile = "files_app_test.png"
const previewImageDimensions = "100 x 100"
const textFile = "text.txt"

// chrome.automation roles
const button = "button"
const staticText = "staticText"

// on screen elements
const downloads = "Downloads"
const more = "More…"
const myFiles = "My files"
const newFolder = "New folder"
const open = "Open"

// RunTest loads the file app and executes a smoke test
func RunTest(ctx context.Context, s *testing.State, cr *chrome.Chrome, previewImage bool) {
	testFileLocation := filepath.Join(downloadPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatal("Failed to create file ", testFileLocation, ": ", err)
	}
	defer os.Remove(testFileLocation)

	if previewImage {
		image, err := ioutil.ReadFile(s.DataPath(previewImageFile))
		if err != nil {
			s.Fatal("Failed to load image ", s.DataPath(previewImageFile), ": ", err)
		}
		imageFileLocation := filepath.Join(downloadPath, previewImageFile)
		if err = ioutil.WriteFile(imageFileLocation, image, 0644); err != nil {
			s.Fatal("Failed to create file ", imageFileLocation, ": ", err)
		}
		defer os.Remove(imageFileLocation)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err = tconn.Exec(ctx, fmt.Sprintf("chrome.autotestPrivate.launchApp(%q, function(){})", filesAppID)); err != nil {
		logDebugInfo(ctx, tconn, s)
		s.Fatal("Failed to launch files app: ", err)
	}

	grabChromeAutomationRoot(ctx, tconn, s)
	clickElement(ctx, tconn, staticText, downloads, s, time.Minute)
	waitForElement(ctx, tconn, staticText, textFile, s, 10*time.Second)

	clickElement(ctx, tconn, button, more, s, 10*time.Second)
	waitForElement(ctx, tconn, staticText, newFolder, s, 10*time.Second)

	if previewImage {
		clickElement(ctx, tconn, staticText, previewImageFile, s, 10*time.Second)
		waitForElement(ctx, tconn, button, open, s, 10*time.Second)

		kb, err := input.Keyboard(ctx)
		if err != nil {
			logDebugInfo(ctx, tconn, s)
			s.Fatal("Failed to get keyboard: ", err)
		}
		defer kb.Close()

		setNavigationOnElement(ctx, tconn, staticText, previewImageFile, s, 10*time.Second)
		waitForElement(ctx, tconn, staticText, "Selected "+previewImageFile+".", s, 10*time.Second)
		if err = kb.Accel(ctx, "Tab"); err != nil {
			logDebugInfo(ctx, tconn, s)
			s.Fatal("Failed to press tab key: ", err)
		}
		testing.Sleep(ctx, time.Second) // Fix flackiness on slow devices
		if err = kb.Accel(ctx, "Space"); err != nil {
			logDebugInfo(ctx, tconn, s)
			s.Fatal("Failed to press space key: ", err)
		}
		waitForElement(ctx, tconn, staticText, previewImageDimensions, s, 10*time.Second)
	}
}

func grabChromeAutomationRoot(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	if err := tconn.Exec(ctx, "var root; chrome.automation.getDesktop(r => root = r);"); err != nil {
		logDebugInfo(ctx, tconn, s)
		s.Fatal("Failed to grab chrome.automation root: ", err)
	}
}

func clickElement(ctx context.Context, tconn *chrome.Conn, role string, name string, s *testing.State, timeout time.Duration) {
	waitForElement(ctx, tconn, role, name, s, timeout)
	clickQuery := fmt.Sprintf("var element = root.find({attributes: {role: %q, name: %q}}); element.doDefault();", role, name)
	if err := tconn.Exec(ctx, clickQuery); err != nil {
		logDebugInfo(ctx, tconn, s)
		s.Fatalf("Failed to click element {role: %q, name: %q}: %s", role, name, err)
	}
}

func waitForElement(ctx context.Context, tconn *chrome.Conn, role string, name string, s *testing.State, timeout time.Duration) {
	findQuery := fmt.Sprintf("root.find({attributes: {role: %q, name: %q}})", role, name)
	if err := tconn.WaitForExprWithTimeoutFailOnErr(ctx, findQuery, timeout); err != nil {
		logDebugInfo(ctx, tconn, s)
		s.Fatalf("Failed to wait for element {role: %q, name: %q}: %s", role, name, err)
	}
}

func setNavigationOnElement(ctx context.Context, tconn *chrome.Conn, role string, name string, s *testing.State, timeout time.Duration) {
	waitForElement(ctx, tconn, role, name, s, timeout)
	navQuery := fmt.Sprintf("var element = root.find({attributes: {role: %q, name: %q}}); element.setSequentialFocusNavigationStartingPoint();", role, name)
	if err := tconn.Exec(ctx, navQuery); err != nil {
		logDebugInfo(ctx, tconn, s)
		s.Fatalf("Failed to set navigation point on element {role: %q, name: %q}: %s", role, name, err)
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
