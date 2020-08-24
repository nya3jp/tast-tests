// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	utf8Data = "Some data that gets copied 🔥 ❄"

	// copyApplet is the data dependency needed to run a copy operation.
	copyApplet      = "copy_applet.py"
	copyAppletDest  = "/home/testuser/copy_applet.py"
	copyAppletTitle = "gtk3_copy_demo"

	// pasteApplet is the data dependency needed to run a paste operation.
	pasteApplet      = "paste_applet.py"
	pasteAppletDest  = "/home/testuser/paste_applet.py"
	pasteAppletTitle = "gtk3_paste_demo"
)

// CopyConfig holds the configuration for the copy half of the test.
type copyConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// waylandCopyConfig is the configuration needed to test copying from
// a wayland application.
var waylandCopyConfig = &copyConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", copyAppletDest},
}

// x11CopyConfig is the configuration needed to test copying from
// an X11 application.
var x11CopyConfig = &copyConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", copyAppletDest},
}

// PasteConfig holds the configuration for the paste half of the test.
type pasteConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// waylandPasteConfig is the configuration needed to test pasting into
// a wayland application.
var waylandPasteConfig = &pasteConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", pasteAppletDest},
}

// x11PasteConfig is the configuration needed to test pasting into
// a x11 application.
var x11PasteConfig = &pasteConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", pasteAppletDest},
}

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	Copy  *copyConfig
	Paste *pasteConfig
}

func init() {

	testing.AddTest(&testing.Test{
		Func:     CopyPaste,
		Desc:     "Test copy paste functionality",
		Contacts: []string{"sidereal@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{copyApplet, pasteApplet},
		// Test every combination of:
		//   * Copy from Wayland|X11
		//   * Copy to Wayland|X11
		Params: crostini.MakeTestParamsFromList([]crostini.Param{
			{
				Name: "wayland_to_wayland",
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: waylandPasteConfig,
				},
			},
			{
				Name: "wayland_to_x11",
				Val: testParameters{
					Copy:  waylandCopyConfig,
					Paste: x11PasteConfig,
				},
			},
			{
				Name: "x11_to_wayland",
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: waylandPasteConfig,
				},
			},
			{
				Name: "x11_to_x11",
				Val: testParameters{
					Copy:  x11CopyConfig,
					Paste: x11PasteConfig,
				},
			}}),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyPaste(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	param := s.Param().(testParameters)
	tconn := pre.TestAPIConn
	cont := pre.Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Clean up the home directory in the end.
	defer func() {
		if err := cont.Cleanup(ctx, "."); err != nil {
			s.Error("Failed to remove all files in home directory in the container: ", err)
		}
	}()

	s.Log("Installing GTK3 dependencies")
	cmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install required dependencies: ", err)
	}

	s.Log("Copying testing applets to container")
	if err := cont.PushFile(ctx, s.DataPath(copyApplet), copyAppletDest); err != nil {
		s.Fatal("Failed to push copy applet to container: ", err)
	}
	if err := cont.PushFile(ctx, s.DataPath(pasteApplet), pasteAppletDest); err != nil {
		s.Fatal("Failed to push paste applet to container: ", err)
	}

	// Add the names of the backends used by each part of the test to differentiate the data used by each test run.
	copiedData := fmt.Sprintf("%v to %v %s", param.Copy.gdkBackend, param.Paste.gdkBackend, utf8Data)

	// The copy event happens at some indeterminate time after the
	// copy applet receives a key press. To be sure we get that event
	// we have to start listening for it before that point.
	// Here, wrapping the promise by a closure in order not to be
	// awaited at this moment.
	var waiting chrome.JSObject
	if err := tconn.Eval(ctx, `(p => () => p)(new Promise((resolve) => {
		  const listener = (e) => {
		    chrome.autotestPrivate.onClipboardDataChanged.removeListener(listener);
		    resolve();
		  };
		  chrome.autotestPrivate.onClipboardDataChanged.addListener(listener);
		}))`, &waiting); err != nil {
		s.Fatal("Failed to set listener for 'copy' event: ", err)
	}
	defer waiting.Release(ctx)
	if _, err := crostini.RunWindowedApp(ctx, tconn, cont, pre.Keyboard, 120*time.Second, func(ctx context.Context) error {
		// Unwrap the promise to wait its settled state.
		return tconn.Call(ctx, nil, `p => p()`, &waiting)
	}, true, copyAppletTitle, append(param.Copy.cmdArgs, copiedData)); err != nil {
		s.Fatal("Failed to run copy applet: ", err)
	}

	output, err := crostini.RunWindowedApp(ctx, tconn, cont, pre.Keyboard, 30*time.Second, nil, false, pasteAppletTitle, param.Paste.cmdArgs)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if output != copiedData {
		s.Fatalf("Unexpected paste output: got %q, want %q", output, copiedData)
	}
}
