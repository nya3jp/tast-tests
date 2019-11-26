// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioRecordSanity,
		Desc:         "Test Apps can record audio to devices",
		Contacts:     []string{"judyhsiao@google.com", "chromeos-audio-bugs@google.com"},
		SoftwareDeps: []string{"android_p_both", "chrome"},
		Data:         []string{"ArcAudioTest.apk"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline"},
	})
}

//AudioRecordSanity testa android can record bytes without exception
func AudioRecordSanity(ctx context.Context, s *testing.State) {

	const (
		apk = "ArcAudioTest.apk"
		pkg = "org.chromium.arc.testapp.arcaudiotestapp"
		cls = "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity"
	)

	a := s.PreValue().(arc.PreData).ARC
	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Granting permission")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.RECORD_AUDIO").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant RECORD_AUDIO permission: ", err)
	}

	s.Log("NewActivity")
	act, err := arc.NewActivity(a, pkg, cls)
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
