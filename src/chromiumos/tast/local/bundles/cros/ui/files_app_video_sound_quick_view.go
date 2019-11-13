// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppVideoSoundQuickView,
		Desc: "Tests that sound plays and stops in video QuickView within the Files app",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.mp4"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesAppVideoSoundQuickView(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Setup the test video.
	const previewVideoFile = "files_app_test.mp4"
	fileLocation := filepath.Join(filesapp.DownloadPath, previewVideoFile)
	if err := fsutil.CopyFile(s.DataPath(previewVideoFile), fileLocation); err != nil {
		s.Fatalf("Failed to copy the test video to %s: %s", fileLocation, err)
	}
	defer os.Remove(fileLocation)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Close(ctx)

	// Open the Downloads folder and check for the test image.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}
	if err := files.WaitForFile(ctx, previewVideoFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test image failed: ", err)
	}

	// Open QuickView for the test video.
	if err := files.OpenQuickView(ctx, previewVideoFile); err != nil {
		s.Fatal("Openning QuickView failed: ", err)
	}

	// Wait for the test video to appear.
	params := ui.FindParams{
		Role: "video",
	}
	if err := files.Root.WaitForDescendantAdded(ctx, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for video failed: ", err)
	}

	// Command to check if audio currently playing.
	cmd := `cat /proc/asound/card*/pcm*/sub*/status | grep -q 'state: RUNNING' && printf "Playing" || printf "Stopped"`

	// Ensure sound has started playing.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return err
		}
		if string(out) != "Playing" {
			return errors.New("sound not playing")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Playing audio failed: ", err)
	}

	// Hit the back button.
	params = ui.FindParams{
		Name: "Back",
		Role: "button",
	}
	back, err := files.Root.GetDescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Waiting for back button failed: ", err)
	}
	defer back.Release(ctx)
	if err := back.LeftClick(ctx); err != nil {
		s.Fatal("Clicking back button failed: ", err)
	}

	// Ensure that sound stops playing.
	s.Log("Waiting for sound to stop running. This can be a bit slow")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return err
		}
		if string(out) != "Stopped" {
			return errors.New("sound still playing")
		}
		return nil
	}, &testing.PollOptions{}); err != nil {
		s.Fatal("Stopping audio failed: ", err)
	}
}
