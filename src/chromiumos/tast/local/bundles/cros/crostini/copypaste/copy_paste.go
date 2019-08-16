// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package copypaste

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	utf8Data = "Some data that gets copied 🔥 ❄"

	// CopyApplet is the data dependency needed to run a copy operation
	CopyApplet      = "copy_applet.py"
	copyAppletDest  = "/home/testuser/copy_applet.py"
	copyAppletTitle = "gtk3_copy_demo"

	// PasteApplet is the data dependency needed to run a paste operation
	PasteApplet     = "paste_applet.py"
	pasteAppletDest = "/home/testuser/paste_applet.py"
)

// CopyConfig holds the configuration for the copy half of the test.
type CopyConfig struct {
	gdkBackend string
	cmdArgs    []string
	data       string
}

// WaylandCopyConfig is the configuration needed to test copying from
// a wayland application.
var WaylandCopyConfig = &CopyConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", copyAppletDest},
	data:       utf8Data,
}

// X11CopyConfig is the configuration needed to test copying from
// an X11 application.
var X11CopyConfig = &CopyConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", copyAppletDest},
	data:       utf8Data,
}

// PasteConfig holds the configuration for the paste half of the test.
type PasteConfig struct {
	gdkBackend   string
	cmdArgs      []string
	expectedData string
}

// WaylandPasteConfig is the configuration needed to test pasting into
// a wayland application.
var WaylandPasteConfig = &PasteConfig{
	gdkBackend:   "wayland",
	cmdArgs:      []string{"env", "GDK_BACKEND=wayland", "python3", pasteAppletDest},
	expectedData: utf8Data,
}

// X11PasteConfig is the configuration needed to test pasting into
// a x11 application.
var X11PasteConfig = &PasteConfig{
	gdkBackend:   "x11",
	cmdArgs:      []string{"env", "GDK_BACKEND=x11", "python3", pasteAppletDest},
	expectedData: utf8Data,
}

// TestParameters contains all the data needed to run a single test iteration
type TestParameters struct {
	Copy  *CopyConfig
	Paste *PasteConfig
}

// RunTest Runs a copy paste test with the supplied parameters.
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont *vm.Container, copy *CopyConfig, paste *PasteConfig) {

	s.Log("Installing GTK3 dependencies")
	cmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install required dependencies: ", err)
	}

	s.Log("Copying testing applets to container")
	if err := cont.PushFile(ctx, s.DataPath(CopyApplet), copyAppletDest); err != nil {
		s.Fatal("Failed to push copy applet to container: ", err)
	}
	if err := cont.PushFile(ctx, s.DataPath(PasteApplet), pasteAppletDest); err != nil {
		s.Fatal("Failed to push paste applet to container: ", err)
	}

	// Add the names of the backends used by each part of the test to differentiate the data used by each test run
	copyData := copy.gdkBackend + " to " + paste.gdkBackend + ": " + copy.data
	pasteData := copy.gdkBackend + " to " + paste.gdkBackend + ": " + paste.expectedData

	func() {
		const timeout = 5 * time.Second
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		s.Logf("Starting copy application for %v backend", copy.gdkBackend)
		args := append(copy.cmdArgs, copyData)
		// TODO before submitting: Find out why the application doesn't close when it's timeout expires
		cmd := cont.Command(ctx, args...)
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to start copy application: ", err)
		}
		defer cmd.Wait(testexec.DumpLogOnError)

		size, err := crostini.PollWindowSize(ctx, tconn, copyAppletTitle)
		if err != nil {
			s.Fatalf("Failed to get size of window %q: %v", copyAppletTitle, err)
		}
		s.Logf("Window %q size is %v", copyAppletTitle, size)

		keyboard, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get keyboard device: ", err)
		}
		defer keyboard.Close()

		s.Logf("Closing %q with keypress", copyAppletTitle)
		keyboard.Type(ctx, " ")
	}()

	s.Logf("Window %q closed", copyAppletTitle)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	s.Logf("Starting paste application for %v backend", paste.gdkBackend)
	cmd = cont.Command(ctx, paste.cmdArgs...)
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if string(output) != pasteData {
		s.Fatalf("Paste output was %q, expected %q", string(output), pasteData)
	}
}
