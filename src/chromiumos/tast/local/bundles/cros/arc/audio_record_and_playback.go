// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil/voicerecorder"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioRecordAndPlayback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Records audio via ARC++ app Voice Recorder and verifies that it can playback the recorded audio file",
		Contacts:     []string{"sun.tsai@cienet.com", "alfredyu@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      mtbf.ArcLoginReuseFixture,
		Timeout:      5 * time.Minute,
	})
}

// AudioRecordAndPlayback records audio via ARC++ app Voice Recorder and verifies that it can playback the recorded audio file..
func AudioRecordAndPlayback(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	recorder, err := mtbf.NewRecorder(ctx)
	if err != nil {
		s.Fatal("Failed to start record performance: ", err)
	}
	defer recorder.Record(cleanupCtx, s.OutDir())

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	vr, err := voicerecorder.New(ctx, kb, tconn, a)
	if err != nil {
		s.Fatal("Failed to create arc resource: ", err)
	}
	defer vr.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	s.Log("Launching the ARC++ app: ", vr.AppName())
	if err := vr.Launch(ctx); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	if err := vr.UpdateOutDir(ctx); err != nil {
		s.Fatalf("Failed to update the output dir of ARC++ app %q: %v", vr.AppName(), err)
	}

	s.Log("Recording audio")
	audioName, err := vr.RecordAudio(ctx)
	if err != nil {
		s.Fatal("Failed to record audio: ", err)
	}
	defer vr.DeleteAudio(audioName)

	s.Log("Playing back the recorded audio")
	if err := vr.PlayFile(audioName)(ctx); err != nil {
		s.Fatal("Failed to play the record: ", err)
	}
}
