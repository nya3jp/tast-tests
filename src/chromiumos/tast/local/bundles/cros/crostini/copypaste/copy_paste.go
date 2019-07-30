// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package copypaste

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// CopyConfig holds the configuration for the copy half of the test.
type CopyConfig struct {
	Name        string
	WindowTitle string
	AppPath     string
	MimeType    string
	Data        string
}

// WaylandCopyConfig is the configuration needed to test copying from
// a wayland applicaiton.
var WaylandCopyConfig = &CopyConfig{
	Name:        "wayland",
	WindowTitle: "Wayland Copy Demo",
	AppPath:     "/opt/google/cros-containers/bin/wayland_copy_demo",
	MimeType:    "text/plain;charset=utf-8",
	Data:        "Some data that gets copied",
}

// PasteConfig holds the configuration for the paste half of the test.
type PasteConfig struct {
	Name         string
	AppPath      string
	MimeType     string
	ExpectedData string
}

// WaylandPasteConfig is the configuration needed to test pasting into
// a wayland applicaiton.
var WaylandPasteConfig = &PasteConfig{
	Name:         "wayland",
	AppPath:      "/opt/google/cros-containers/bin/wayland_paste_demo",
	MimeType:     "text/plain;charset=utf-8",
	ExpectedData: "Some data that gets copied",
}

// RunTest Run a copy paste test with the supplied parameters
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont vm.Container, copy CopyConfig, paste PasteConfig) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard device: ", err)
	}
	defer keyboard.Close()

	s.Logf("Starting %v copy application", copy.Name)
	cmd := cont.Command(ctx, copy.AppPath, copy.MimeType, copy.Data)
	err = cmd.Start()
	if err != nil {
		s.Fatal("Failed to start copy application: ", err)
	}

	size, err := crostini.PollWindowSize(ctx, tconn, copy.WindowTitle)
	if err != nil {
		s.Fatalf("Failed to get size of window %q: %v", copy.WindowTitle, err)
	}
	s.Logf("Window %q size is %v", copy.WindowTitle, size)

	s.Logf("Closing %q with keypress", copy.WindowTitle)
	keyboard.Type(ctx, " ")

	err = cmd.Wait(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to wait on copy application: ", err)
	}
	s.Logf("Window %q closed", copy.WindowTitle)

	s.Logf("Starting %v paste application", paste.Name)
	cmd = cont.Command(ctx, paste.AppPath, paste.MimeType)
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if string(output) != paste.ExpectedData {
		s.Fatalf("Paste output was %q, expected %q", string(output), paste.ExpectedData)
	}
}
