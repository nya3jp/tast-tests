// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	pasteApplet = "secure_paste.py"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SecurePaste,
		Desc:         "Verifies that background crostini apps can not access the clipboard (for pasting)",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact, pasteApplet, "secure_blocker.html"},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func SecurePaste(ctx context.Context, s *testing.State) {
	const (
		knownErrorText = "application is still visible"
	)
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := pre.Container

	aptCmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := aptCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %s: %v", shutil.EscapeSlice(aptCmd.Args), err)
	}

	// Set the clipboard data before running the application.
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)("initial")`, nil); err != nil {
		s.Fatal("Failed to set the initial clipboard text: ", err)
	}

	// Launch the paste app.
	if err := cont.PushFile(ctx, s.DataPath(pasteApplet), "/home/testuser/"+pasteApplet); err != nil {
		s.Fatalf("Failed to push %v to container: %v", pasteApplet, err)
	}
	appID, exitCallback, err := crostini.LaunchCrostiniApp(ctx, tconn, cont.Command(ctx, "env", "GDK_BACKEND=wayland", "python3", pasteApplet))
	if err != nil {
		s.Fatal("Failed to launch crostini app: ", err)
	}
	s.Log("Launched crostini app with ID: ", appID)
	defer exitCallback()

	// Bring up a webpage to block the crostini app.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	conn, err := cr.NewConn(ctx, server.URL+"/secure_blocker.html")
	if err != nil {
		s.Fatal("Failed to open a blocker: ", err)
	}
	defer conn.Close()
	if err := crostini.MatchScreenshotDominantColor(ctx, cr, colorcmp.RGB(0, 0, 0), filepath.Join(s.OutDir(), "screenshot.png")); err != nil {
		s.Fatal("Failed during screenshot check: ", err)
	}

	// Change the clipboard data while the app is running.
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)("exit")`, nil); err != nil {
		s.Fatal("Failed to change the clipboard text: ", err)
	}

	// While the blocker is active, the application should NOT see "exit" is on the clipboard, so it won't exit.
	checkVisible := func(ctx context.Context) error {
		if visible, err := ash.AppShown(ctx, tconn, appID); err != nil {
			return err
		} else if visible {
			return errors.New(knownErrorText)
		}
		return nil
	}
	if err := testing.Poll(ctx, checkVisible, &testing.PollOptions{Timeout: 5 * time.Second}); err == nil {
		s.Fatal("Failed to block clipboard contents while inactive")
	} else if !strings.Contains(err.Error(), knownErrorText) {
		s.Fatal("Failed to ensure the clipboard was blocked: ", err)
	} else if err := conn.CloseTarget(ctx); err != nil {
		s.Fatal("Failed to close the blocker: ", err)
	}

	//Now the blocker is removed, the application can paste the clipboard and exit itself.
	if err := testing.Poll(ctx, checkVisible, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to detect clipboard contents while active: ", err)
	}
}
