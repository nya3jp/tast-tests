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
	utf8PlainText = "text/plain;charset=utf-8"
	utf8Data      = "Some data that gets copied"
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
// a wayland application.
var WaylandCopyConfig = &CopyConfig{
	Name:        "wayland",
	WindowTitle: "Wayland Copy Demo",
	AppPath:     "/opt/google/cros-containers/bin/wayland_copy_demo",
	MimeType:    utf8PlainText,
	Data:        utf8Data,
}

// PasteConfig holds the configuration for the paste half of the test.
type PasteConfig struct {
	Name         string
	AppPath      string
	MimeType     string
	ExpectedData string
}

// WaylandPasteConfig is the configuration needed to test pasting into
// a wayland application.
var WaylandPasteConfig = &PasteConfig{
	Name:         "wayland",
	AppPath:      "/opt/google/cros-containers/bin/wayland_paste_demo",
	MimeType:     utf8PlainText,
	ExpectedData: utf8Data,
}

// RunTest Runs a copy paste test with the supplied parameters.
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont *vm.Container, copy *CopyConfig, paste *PasteConfig) {

	func() {
		const timeout = 5 * time.Second
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		s.Logf("Starting %v copy application", copy.Name)
		args := []string{copy.AppPath, copy.MimeType, copy.Data}
		cmd := cont.Command(ctx, args...)
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to start copy application: ", err)
		}
		defer cmd.Wait(testexec.DumpLogOnError)

		size, err := crostini.PollWindowSize(ctx, tconn, copy.WindowTitle)
		if err != nil {
			s.Fatalf("Failed to get size of window %q: %v", copy.WindowTitle, err)
		}
		s.Logf("Window %q size is %v", copy.WindowTitle, size)

		keyboard, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get keyboard device: ", err)
		}
		defer keyboard.Close()

		s.Logf("Closing %q with keypress", copy.WindowTitle)
		keyboard.Type(ctx, " ")
	}()

	s.Logf("Window %q closed", copy.WindowTitle)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	s.Logf("Starting %v paste application", paste.Name)
	cmd := cont.Command(ctx, paste.AppPath, paste.MimeType)
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run paste application: ", err)
	}

	if string(output) != paste.ExpectedData {
		s.Fatalf("Paste output was %q, expected %q", string(output), paste.ExpectedData)
	}
}
