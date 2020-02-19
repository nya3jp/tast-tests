// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type testParameters struct {
	Permission string
	Class      string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AudioSanity,
		Desc: "Audio sanity test for arc",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"judyhsiao@chromium.org",         // Author
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcAudioTest.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "playback",
				Val: testParameters{
					Class: "org.chromium.arc.testapp.arcaudiotestapp.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               arc.Booted(),
			},
			{
				Name: "playback_vm",
				Val: testParameters{
					Class: "org.chromium.arc.testapp.arcaudiotestapp.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm_p"},
				Pre:               arc.VMBooted(),
			},
			{
				Name: "record",
				Val: testParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               arc.Booted(),
			},
			{
				Name: "record_vm",
				Val: testParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm_p"},
				Pre:               arc.VMBooted(),
			},
		},
	})
}

func AudioSanity(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcAudioTest.apk"
		pkg = "org.chromium.arc.testapp.arcaudiotestapp"
		// UI IDs in the app.
		idPrefix = pkg + ":id/"
		resultID = idPrefix + "test_result"
		logID    = idPrefix + "test_result_log"
	)
	param := s.Param().(testParameters)

	a := s.PreValue().(arc.PreData).ARC
	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if param.Permission != "" {
		s.Log("Granting permission")
		if err := a.Command(ctx, "pm", "grant", pkg, param.Permission).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to grant permission: ", err)
		}
	}

	s.Log("New Activity")
	act, err := arc.NewActivity(a, pkg, param.Class)
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	// Launch the activity.
	s.Log("Start Activity")
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	if err := d.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, 20*time.Second); err != nil {
		s.Fatal("Timed out for waiting result updated: ", err)
	}

	// Test result can be either '0' or '1', where '0' means fail and '1'
	// means pass.
	if result, err := d.Object(ui.ID(resultID)).GetText(ctx); err != nil {
		s.Fatal("Failed to get the result: ", err)
	} else if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := d.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			s.Fatal("Failed to get failure reason: ", err)
		}
		s.Error("Test failed: ", reason)
	}
}
