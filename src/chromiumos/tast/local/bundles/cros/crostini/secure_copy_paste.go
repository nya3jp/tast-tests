// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type copyPasteAction int

const (
	copying copyPasteAction = iota
	pasting
	blockerTitle string = "secure_blocker.html"
)

type secureCopyPasteConfig struct {
	backend string
	app     string
	action  copyPasteAction
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     SecureCopyPaste,
		Desc:     "Verifies that background crostini apps can not access the clipboard",
		Contacts: []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Params: []testing.Param{{
			Name:      "copy_wayland_artifact",
			ExtraData: []string{"secure_copy.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:      "copy_wayland_artifact_unstable",
			ExtraData: []string{"secure_copy.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "copy_x11_artifact",
			ExtraData: []string{"secure_copy.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:      "copy_x11_artifact_unstable",
			ExtraData: []string{"secure_copy.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "paste_wayland_artifact",
			ExtraData: []string{"secure_paste.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:      "paste_wayland_artifact_unstable",
			ExtraData: []string{"secure_paste.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "paste_x11_artifact",
			ExtraData: []string{"secure_paste.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraSoftwareDeps: []string{"crostini_stable"},
		}, {
			Name:      "paste_x11_artifact_unstable",
			ExtraData: []string{"secure_paste.py", crostini.ImageArtifact},
			Pre:       crostini.StartedByArtifact(),
			Timeout:   7 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraSoftwareDeps: []string{"crostini_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "copy_wayland_download",
			ExtraData: []string{"secure_copy.py"},
			Pre:       crostini.StartedByDownload(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "copy_x11_download",
			ExtraData: []string{"secure_copy.py"},
			Pre:       crostini.StartedByDownload(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "paste_wayland_download",
			ExtraData: []string{"secure_paste.py"},
			Pre:       crostini.StartedByDownload(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "paste_x11_download",
			ExtraData: []string{"secure_paste.py"},
			Pre:       crostini.StartedByDownload(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "copy_wayland_download_buster",
			ExtraData: []string{"secure_copy.py"},
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "copy_x11_download_buster",
			ExtraData: []string{"secure_copy.py"},
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_copy.py",
				action:  copying,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "paste_wayland_download_buster",
			ExtraData: []string{"secure_paste.py"},
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "paste_x11_download_buster",
			ExtraData: []string{"secure_paste.py"},
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			Val: secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_paste.py",
				action:  pasting,
			},
			ExtraAttr: []string{"informational"},
		}},
		Data:         []string{blockerTitle},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// forceClipboard is a stronger version of the setClipboardTextData api, which
// repeatedly sets/checks the clipboard data to ensure that the requested value
// is on there. We need this because the applications under test are fighting
// for clipboard control.
func forceClipboard(ctx context.Context, tconn *chrome.TestConn, data string) error {
	setClipboardPromise := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setClipboardTextData)(%q)`, data)
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.EvalPromise(ctx, setClipboardPromise, nil); err != nil {
			return err
		}
		var clipData string
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return err
		}
		if clipData != data {
			return errors.Errorf("clipboard data missmatch: got %q, want %q", clipData, data)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

func SecureCopyPaste(ctx context.Context, s *testing.State) {
	conf := s.Param().(secureCopyPasteConfig)
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := pre.Container

	// Initialize the clipboard data before the test.
	if err := forceClipboard(ctx, tconn, ""); err != nil {
		s.Fatal("Failed to set clipboard data: ", err)
	}

	// Install dependencies.
	aptCmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := aptCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %s: %v", shutil.EscapeSlice(aptCmd.Args), err)
	}

	// Launch the app.
	if err := cont.PushFile(ctx, s.DataPath(conf.app), "/home/testuser/"+conf.app); err != nil {
		s.Fatalf("Failed to push %v to container: %v", conf.app, err)
	}
	appID, exitCallback, err := crostini.LaunchGUIApp(ctx, tconn, cont.Command(ctx, "env", "GDK_BACKEND="+conf.backend, "python3", conf.app))
	if err != nil {
		s.Fatal("Failed to launch crostini app: ", err)
	}
	defer exitCallback()
	s.Log("Launched crostini app with ID: ", appID)

	// Bring up a webpage to block the crostini app. We use the MatchScreenshotDominantColor
	// trick because we have need the page to complete fading in animations before it can
	// block other applications' access to the clipboard.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	conn, err := cr.NewConn(ctx, server.URL+"/"+blockerTitle)
	if err != nil {
		s.Fatal("Failed to open a blocker: ", err)
	}
	defer conn.Close()
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to retrieve currently opened windows: ", err)
	}
	// Maximize the blocker to ensure our screenshot dominant colour condition succeeds.
	// GetAllWindows returns windows by their stacking order, so ws[0] is the foregrounded window.
	maximized := false
	for _, w := range ws {
		if strings.Contains(w.Title, blockerTitle) {
			ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventMaximize)
			maximized = true
			break
		}
	}
	if !maximized {
		s.Fatal("Failed to find the secure_blocker window to maximize")
	}
	if err := crostini.MatchScreenshotDominantColor(ctx, cr, colorcmp.RGB(0, 0, 0), filepath.Join(s.OutDir(), "screenshot.png")); err != nil {
		s.Fatal("Failed during screenshot check: ", err)
	}

	// Set the clipboard data now that the blocker is up.
	if err := forceClipboard(ctx, tconn, "secret"); err != nil {
		s.Fatal("Failed to set clipboard data: ", err)
	}

	// Poll the clipboard to make sure it does NOT change.
	var clipboardCheck func(ctx context.Context) error
	if conf.action == copying {
		// For copying, we are checking to see that the app didn't replace the clipboard
		// contents (currently: "secret").
		clipboardCheck = func(ctx context.Context) error {
			var clipData string
			if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
				return err
			} else if clipData == "secret" {
				return errors.New("clipboard data has not been changed")
			}
			return nil
		}
	} else {
		// For pasting, the app will exit itself if it reads "secret" from the clipboard,
		// so we just check to see if it is still running.
		clipboardCheck = func(ctx context.Context) error {
			if visible, err := ash.AppShown(ctx, tconn, appID); err != nil {
				return err
			} else if visible {
				return errors.New("application is still visible")
			}
			return nil
		}
	}

	// First, check that while the blocker is up, the app can not interact with the clipboard.
	if err := testing.Poll(ctx, clipboardCheck, &testing.PollOptions{Timeout: 30 * time.Second}); err == nil {
		s.Fatal("Failed to block clipboard contents while inactive")
	}

	// Remove the blocker.
	if err := conn.CloseTarget(ctx); err != nil {
		s.Fatal("Failed to close the blocker: ", err)
	}

	// Now the blocker is removed, re-run the above theck to ensure that the (now active) app can interact.
	if err := testing.Poll(ctx, clipboardCheck, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to access the clipboard while active: ", err)
	}
}
