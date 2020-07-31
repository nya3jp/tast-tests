// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppAndroidStudio,
		Desc:     "Test android studio in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact_trace",
			Pre:               crostini.StartedTraceVM(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster_disksize5g",
			Pre:     crostini.StartedByDownloadBusterDiskSize5G(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host", "amd64"},
	})
}
func AppAndroidStudio(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()

	// Reboot the container in the end in case any error in the middle and android studio is not closed.
	defer cont.Reboot(cleanupCtx)

	const downloadFile = "android.tar.gz"

	// Install android studio code in container. This is a work around until android studio is pre-installed in a image.
	s.Log("Downloading and installing android studio")
	urlOfAndroidStudio := "https://storage.googleapis.com/chromiumos-test-assets-public/crostini_test_files/android-studio-linux.tar.gz"
	if err := cont.RunMultiCommandsInSequence(ctx, fmt.Sprintf("wget %s -O %s", urlOfAndroidStudio, downloadFile),
		fmt.Sprintf("sudo tar -zxvf %s", downloadFile)); err != nil {
		s.Fatal("Failed to install android studio in container: ", err)
	}

	userName := strings.Split(cr.User(), "@")[0]

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, userName)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	defer terminalApp.Close(cleanupCtx, keyboard)

	// Open android studio.
	if err := terminalApp.RunCommand(ctx, keyboard, "./android-studio/bin/studio.sh &"); err != nil {
		s.Fatal("Failed to start android studio in Terminal: ", err)
	}

	// Find window.
	param := ui.FindParams{
		Name: "Import Android Studio Settings From...",
		Role: ui.RoleTypeWindow,
	}
	if _, err := ui.FindWithTimeout(ctx, tconn, param, 30*time.Second); err != nil {
		s.Fatal("Failed to find android studio window: ", err)
	}

	//TODO(jinrongwu): UI test on android studio code.
}
