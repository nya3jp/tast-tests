// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

type params struct {
	effect string
	width  string
	height string
	image  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: EffectsBenchmark,
		Desc: "Runs the Effects benchmark",
		Contacts: []string{
			"jakebarnes@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_feature_effects"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "blur_1080p",
				ExtraData: []string{"wfh_1080p.nv12"},
				Val: params{
					effect: "blur",
					width:  "1920",
					height: "1080",
					image:  "wfh_1080p.nv12",
				},
			},
		},
	})
}

func EffectsBenchmark(ctx context.Context, s *testing.State) {
	p, ok := s.Param().(params)
	if !ok {
		s.Fatal("Failed to convert test params")
	}

	logFile, err := os.OpenFile(
		filepath.Join(s.OutDir(), "effects_benchmark.log"),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		s.Fatal("Failed to create log file: ", logFile)
	}
	defer logFile.Close()
	cmd := testexec.CommandContext(ctx,
		"effects_benchmark",
		"--effect="+p.effect,
		"--image="+s.DataPath(p.image),
		"--width="+p.width,
		"--height="+p.height)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to run benchmark: ", err)
	}
}
