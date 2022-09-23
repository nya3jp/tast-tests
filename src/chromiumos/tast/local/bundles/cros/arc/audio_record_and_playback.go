// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/voicerecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

const recordingDuration = 5 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioRecordAndPlayback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Records audio via ARC++ app Voice Recorder and verifies that it can playback the recorded audio file",
		Contacts:     []string{"sun.tsai@cienet.com", "alfredyu@cienet.com", "cienet-development@googlegroups.com"},
		// Purposely leave the empty Attr here. MTBF tests are not included in mainline or crosbolt for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		VarDeps:      []string{"uidetection.key_type", "uidetection.key", "uidetection.server"},
		Data:         []string{voicerecorder.PlayingIcon},
		Fixture:      mtbf.LoginReuseFixture,
		Timeout:      3*time.Minute + apputil.InstallationTimeout,
	})
}

// AudioRecordAndPlayback records audio via ARC++ app Voice Recorder and verifies that it can playback the recorded audio file..
func AudioRecordAndPlayback(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	a := s.FixtValue().(*mtbf.FixtValue).ARC

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
	if err := vr.RecordAudioFor(cr, recordingDuration)(ctx); err != nil {
		s.Fatal("Failed to record audio: ", err)
	}
	defer vr.DeleteLatestRecord(cleanupCtx, cr)

	s.Log("Playing back the recorded audio")
	ud := uidetection.New(tconn, s.RequiredVar("uidetection.key_type"), s.RequiredVar("uidetection.key"), s.RequiredVar("uidetection.server"))
	if err := vr.PlayLatestRecord(ud, s.DataPath(voicerecorder.PlayingIcon))(ctx); err != nil {
		s.Fatal("Failed to play the record: ", err)
	}
}
