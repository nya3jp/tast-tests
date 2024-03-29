// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GPUEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that Crostini starts with the correct GPU device depending on whether the GPU flag is set or not",
		Contacts:     []string{"clumptini+oncall@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host", "crosvm_gpu"},
		Params: []testing.Param{
			// Parameters generated by gpu_enabled_test.go. DO NOT EDIT.
			{
				Name:              "sw_buster_stable",
				ExtraSoftwareDeps: []string{"crosvm_no_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val:               "llvmpipe",
			}, {
				Name:              "sw_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"crosvm_no_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val:               "llvmpipe",
			}, {
				Name:              "sw_bullseye_stable",
				ExtraSoftwareDeps: []string{"crosvm_no_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val:               "llvmpipe",
			}, {
				Name:              "sw_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"crosvm_no_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val:               "llvmpipe",
			}, {
				Name:              "gpu_buster_stable",
				ExtraSoftwareDeps: []string{"crosvm_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val:               "virgl",
			}, {
				Name:              "gpu_buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"crosvm_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
				Val:               "virgl",
			}, {
				Name:              "gpu_bullseye_stable",
				ExtraSoftwareDeps: []string{"crosvm_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val:               "virgl",
			}, {
				Name:              "gpu_bullseye_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"crosvm_gpu", "dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseye",
				Timeout:           7 * time.Minute,
				Val:               "virgl",
			},
		},
	})
}

func GPUEnabled(ctx context.Context, s *testing.State) {
	cont := s.FixtValue().(crostini.FixtureData).Cont
	expectedDevice := s.Param().(string)

	cmd := cont.Command(ctx, "sh", "-c", "glxinfo -B | grep Device:")
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %q: %v", shutil.EscapeSlice(cmd.Args), err)
	} else {
		output := string(out)
		if !strings.Contains(output, expectedDevice) {
			s.Fatalf("Failed to verify GPU device: got %q; want %q", output, expectedDevice)
		}
		s.Logf("GPU is %q", output)
	}
}
