// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasRecordQuality,
		Desc:         "Verifies recorded samples from CRAS are correct",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_record"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrasRecordQuality(ctx context.Context, s *testing.State) {

	const (
		duration = 1 // second
	)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the normal execution.
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	fileName := filepath.Join(s.OutDir(), "recorded.raw")

	// Record function by CRAS.
	if err := testexec.CommandContext(
		runCtx, "cras_test_client",
		"--capture_file", fileName,
		"--duration", strconv.Itoa(duration),
		"--num_channels", "2",
		"--rate", "48000",
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to record from CRAS: ", err)
	}

	if err := audio.CheckRecordingQuality(ctx, fileName); err != nil {
		s.Error("Failed to check quality: ", err)
	}
}
