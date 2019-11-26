// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
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
		Desc: "Test Apps can play audio to devices",
		Contacts: []string{
			"cychiang@chromium.org",  // Media team
			"judyhsiao@chromium.org", // Author
		},
		SoftwareDeps: []string{"android_p_both", "chrome"},
		Data:         []string{"ArcAudioTest.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name: "playback",
				Val: testParameters{
					Class: "org.chromium.arc.testapp.arcaudiotestapp.TestOutputActivity",
				},
				Pre:     arc.Booted(),
				Timeout: 3 * time.Minute,
			},
			{
				Name: "record",
				Val: testParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity",
				},
				Pre:     arc.Booted(),
				Timeout: 3 * time.Minute,
			},
		},
	})
}

//AudioSanity testa android can playback bytes without exception
func AudioSanity(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcAudioTest.apk"
		pkg = "org.chromium.arc.testapp.arcaudiotestapp"
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
}
