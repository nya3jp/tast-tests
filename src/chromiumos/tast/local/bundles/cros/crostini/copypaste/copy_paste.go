// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package copypaste

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
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
	PasteApplet      = "paste_applet.py"
	pasteAppletDest  = "/home/testuser/paste_applet.py"
	pasteAppletTitle = "gtk3_paste_demo"
)

// CopyConfig holds the configuration for the copy half of the test.
type CopyConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// WaylandCopyConfig is the configuration needed to test copying from
// a wayland application.
var WaylandCopyConfig = &CopyConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", copyAppletDest},
}

// X11CopyConfig is the configuration needed to test copying from
// an X11 application.
var X11CopyConfig = &CopyConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", copyAppletDest},
}

// PasteConfig holds the configuration for the paste half of the test.
type PasteConfig struct {
	gdkBackend string
	cmdArgs    []string
}

// WaylandPasteConfig is the configuration needed to test pasting into
// a wayland application.
var WaylandPasteConfig = &PasteConfig{
	gdkBackend: "wayland",
	cmdArgs:    []string{"env", "GDK_BACKEND=wayland", "python3", pasteAppletDest},
}

// X11PasteConfig is the configuration needed to test pasting into
// a x11 application.
var X11PasteConfig = &PasteConfig{
	gdkBackend: "x11",
	cmdArgs:    []string{"env", "GDK_BACKEND=x11", "python3", pasteAppletDest},
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
	copiedData := copy.gdkBackend + " to " + paste.gdkBackend + ": " + utf8Data

	output, err := crostini.RunWindowedApp(ctx, tconn, cont, 5*time.Second, copyAppletTitle, append(copy.cmdArgs, copiedData))
	if err != nil {
		s.Fatal("Failed to run copy applet: ", err)
	}

	output, err = crostini.RunWindowedApp(ctx, tconn, cont, 5*time.Second, pasteAppletTitle, paste.cmdArgs)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if output != copiedData {
		s.Fatalf("Paste output was %q, expected %q", output, copiedData)
	}
}
