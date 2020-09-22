// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	backgroundJS = "disable_screenshots_extension/background.js"
	contentJS    = "disable_screenshots_extension/content.js"
	manifestJSON = "disable_screenshots_extension/manifest.json"
)

var extensionFiles []string = []string{backgroundJS, contentJS, manifestJSON}

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshotsExtension,
		// TODO(crbug.com/1125556): check whether screenshot can be taken by extensions APIs.
		Desc: "Behavior of the DisableScreenshots policy, check whether screenshot can be taken by pressing hotkeys",
		Contacts: []string{
			"lamzin@google.com", // Test port author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         extensionFiles,
	})
}

func DisableScreenshotsExtension(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "disable_screenshot_extension")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	if err := os.Chown(extDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatalf("Failed to chown %q dir: %v", extDir, err)
	}

	for _, file := range extensionFiles {
		dst := filepath.Join(extDir, filepath.Base(file))
		if err := fsutil.CopyFile(s.DataPath(file), dst); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, extDir, err)
		}
		// if err := os.Chown(dst, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		// 	s.Fatalf("Failed to chown %q file: %v", dst, err)
		// }
	}

	// extID, err := chrome.ComputeExtensionID(extDir)
	// if err != nil {
	// 	s.Fatal("Failed to compute extension ID: ", err)
	// }
	// s.Log("ID: ", extID)

	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		s.Fatal("Failed to create fdms temp dir: ", err)
	}

	testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
	fdms, err := fakedms.New(ctx, tmpdir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies([]policy.Policy{&policy.DisableScreenshots{Val: true}})
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.UnpackedExtension(extDir),
		chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(ctx)

	tabConn, err := cr.NewConn(ctx, "https://google.com")
	if err != nil {
		s.Fatal("Failed to create a tab: ", err)
	}
	defer tabConn.Close()

	// bgURL := chrome.ExtensionBackgroundPageURL(extID)
	// conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	// if err != nil {
	// 	s.Fatal("Failed to connect to extension at %q: %v", bgURL, err)
	// }
	// defer conn.Close()

	if err := tabConn.Eval(ctx, `document.title = "tabCapture"`, nil); err != nil {
		s.Fatal("Failed to execute JS in extension: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}

	if err := keyboard.Accel(ctx, "Ctrl+Shift+Y"); err != nil {
		s.Fatal("Failed to press Ctrl+Shift+Y: ", err)
	}

	// if err := keyboard.Accel(ctx, "Enter"); err != nil {
	// 	s.Fatal("Failed to press Enter: ", err)
	// }

	if err := tabConn.WaitForExpr(ctx, `document.title != "tabCapture"`); err != nil {
		s.Fatal("Failed to execute JS in extension: ", err)
	}

	var title interface{}
	if err := tabConn.Eval(ctx, `document.title`, &title); err != nil {
		s.Fatal("Failed to execute JS in extension: ", err)
	}
	s.Logf("Title: %q", title)
}
