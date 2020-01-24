// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform/memoryuser"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MempressureUser,
		Desc:     "Tests video loading times after creating memory pressure",
		Contacts: []string{"asavery@chromium.org", "chromeos-storage@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  180 * time.Minute,
		Data: []string{
			mempressure.CompressibleData,
			mempressure.WPRArchiveName,
			"memory_user_youtube.apk",
		},
		SoftwareDeps: []string{"android", "chrome"},
	})
}

func MempressureUser(ctx context.Context, s *testing.State) {
	const (
		cls      = "android.intent.action.VIEW"
		apk      = "memory_user_youtube.apk"
		pkg      = "com.google.android.youtube"
		actName  = ".HomeActivity"
		vidLink  = "https://www.youtube.com/watch?v=WS-MfCjzztg"
		vidLink2 = "https://www.youtube.com/watch?v=JE3-LkMqBfM"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Something failed: ", err)
		}
	}

	youtubeFunc := func(a *arc.ARC) {
		device, err := ui.NewDevice(ctx, a)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer device.Close()
		// Mute the DUT
		cmd := testexec.CommandContext(ctx, "cras_test_client", "--mute", "1")
		if err := cmd.Run(); err != nil {
			testing.ContextLog(ctx, "Mute command failed: ", err)
		}

		// Start the first video
		startTime := time.Now()
		if err = a.Command(ctx, "am", "start", "-a", cls, "-d", vidLink, "-n", "com.google.android.youtube/.UrlActivity").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to play video: ", err)
		}

		// Turn autoplay off
		autoP := device.Object(ui.Description("Autoplay"), ui.Clickable(true))
		must(autoP.WaitForExists(ctx, 90*time.Second))
		autoP.Click(ctx)

		// Wait for the first video to finish playing and log the total play/loading time
		done := device.Object(ui.Description("01:30 of 01:30"))
		must(done.WaitForExists(ctx, 180*time.Second))
		loadingTime := time.Since(startTime).Truncate(time.Millisecond)
		s.Log("Loading time is: ", loadingTime)
		s.Log("Video length is: 90s, difference is: ", (loadingTime - 90*time.Second))

		// Start the second video
		startTime = time.Now()
		if err = a.Command(ctx, "am", "start", "-a", cls, "-d", vidLink2, "-n", "com.google.android.youtube/.UrlActivity").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to play music file: ", err)
		}

		// Wait for the second video to finish playing and log the loading time
		done = device.Object(ui.Description("04:24 of 04:24"))
		must(done.WaitForExists(ctx, 300*time.Second))
		loadingTime = time.Since(startTime).Truncate(time.Millisecond)
		s.Log("Loading time is: ", loadingTime)
		s.Log("Video length is: 264s, difference is: ", (loadingTime - 264*time.Second))
	}

	// The android task that will open the youtube app and play 2 videos
	aTask := memoryuser.AndroidTask{APKPath: s.DataPath(apk), APK: apk, Pkg: pkg, ActivityName: actName, TestFunc: youtubeFunc}

	p := &mempressure.RunParameters{
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
	}

	mpTask := memoryuser.MemPressureTask{Params: p}

	memTasks := []memoryuser.MemoryTask{&mpTask, &aTask}
	rp := &memoryuser.RunParameters{
		WPRArchivePath: s.DataPath(mempressure.WPRArchiveName),
		UseARC:         true,
	}
	memoryuser.RunTest(ctx, s, memTasks, rp)
}
