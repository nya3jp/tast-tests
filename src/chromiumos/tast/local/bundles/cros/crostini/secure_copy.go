// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	copyApplet = "secure_copy.py"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SecureCopy,
		Desc:         "Verifies that background crostini apps can not access the clipboard (for copying)",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact, copyApplet, "secure_blocker.html"},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func SecureCopy(ctx context.Context, s *testing.State) {
	const (
		knownErrorText = "clipboard data has not been changed"
	)
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := pre.Container

	aptCmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := aptCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %s: %v", shutil.EscapeSlice(aptCmd.Args), err)
	}

	// Launch the copy app.
	if err := cont.PushFile(ctx, s.DataPath(copyApplet), "/home/testuser/"+copyApplet); err != nil {
		s.Fatalf("Failed to push %v to container: %v", copyApplet, err)
	}
	if id, exitCallback, err := crostini.LaunchCrostiniApp(ctx, tconn, cont.Command(ctx, "env", "GDK_BACKEND=wayland", "python3", copyApplet)); err != nil {
		s.Fatal("Failed to launch crostini app: ", err)
	} else {
		s.Log("Launched crostini app with ID: ", id)
		defer exitCallback()
	}

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

	// Set the clipboard data now that the blocker is up.
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)("fixed")`, nil); err != nil {
		s.Fatal("Failed to set the clipboard text: ", err)
	}

	// Poll the clipboard to make sure it does NOT change.
	var clipData string
	checkClipboard := func(ctx context.Context) error {
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return err
		} else if _, err := strconv.ParseInt(clipData, 10, 64); err != nil {
			return errors.New(knownErrorText)
		}
		return nil
	}
	if err := testing.Poll(ctx, checkClipboard, &testing.PollOptions{Timeout: 5 * time.Second}); err == nil {
		s.Fatalf("Failed to block clipboard contents while inactive: got %q, want \"fixed\"", clipData)
	} else if !strings.Contains(err.Error(), knownErrorText) {
		s.Fatal("Failed to ensure the clipboard was blocked: ", err)
	} else if err := conn.CloseTarget(ctx); err != nil {
		s.Fatal("Failed to close the blocker: ", err)
	}

	if clipData != "fixed" {
		s.Fatalf("Failed to verify clipboard data: got %q, wanted \"fixed\"", clipData)
	}

	// Now the blocker is removed, poll the clipboard to make sure it changes.
	if err := testing.Poll(ctx, checkClipboard, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to change the clipboard while active: ", err)
	}
}
