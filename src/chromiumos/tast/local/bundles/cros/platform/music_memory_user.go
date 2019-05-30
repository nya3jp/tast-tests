// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform/memoryuser"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MusicMemoryUser,
		Desc:         "Tests heavy memory use with Chrome, ARC and VMs running",
		Contacts:     []string{"asavery@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		Data:         []string{"memory_user_play_music.apk", "memory_user_Song.mp3"},
		SoftwareDeps: []string{"android", "chrome_login", "vm_host"},
	})
}

func MusicMemoryUser(ctx context.Context, s *testing.State) {
	const (
		apk          = "memory_user_play_music.apk"
		pkg          = "com.google.android.music"
		activityName = ".ui.HomeActivity"
		cls          = "android.intent.action.VIEW"
		music        = "memory_user_Song.mp3"
		arcMusicPath = "/storage/emulated/0/Song.mp3"
		mPath        = "file:///storage/emulated/0/Song.mp3"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Something failed: ", err)
		}
	}

	arcFunc := func(a *arc.ARC, d *ui.Device) {
		testing.Sleep(ctx, 20*time.Second)
		err := a.PushFile(ctx, s.DataPath(music), arcMusicPath)
		if err != nil {
			s.Fatal("Failed to move music file into arc: ", err)
		}
		if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.READ_EXTERNAL_STORAGE").Run(); err != nil {
			s.Fatal("Failed to play music file: ", err)
		}
		if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.WRITE_EXTERNAL_STORAGE").Run(); err != nil {
			s.Fatal("Failed to play music file: ", err)
		}
		s.Log(s.DataPath(music))
		startTime := time.Now()
		output, err := a.Command(ctx, "am", "start", "-a", cls, "-d", mPath, "-t", "audio/mp3", "-n", "com.google.android.music/.AudioPreview", "-W").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to play music file: ", err)
		}
		s.Log(output)
		pBut := d.Object(ui.Clickable(true), ui.Description("Pause"))
		must(pBut.WaitForExists(ctx, 120*time.Second))

		pBut = d.Object(ui.Clickable(true), ui.Description("Play"))
		must(pBut.WaitForExists(ctx, 120*time.Second))
		loadingTime := time.Now().Sub(startTime).Seconds()
		s.Log("Loading time is: ", loadingTime)

	}

	aTask := memoryuser.AndroidTask{APKPath: s.DataPath(apk), APK: apk, Pkg: pkg, ActivityName: activityName, TestFunc: arcFunc}
	memTasks := []memoryuser.MemoryTask{&aTask}
	memoryuser.RunTest(ctx, s, memTasks)
}
