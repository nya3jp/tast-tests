// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GPUEnabled,
		Desc:         "Tests that Crostini starts with the correct GPU device depending on whether the GPU flag is set or not",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host", "crosvm_gpu"},
		Params: crostini.MakeTestParamsFromList([]crostini.Param{
			{
				Name:              "sw",
				Val:               "llvmpipe",
				ExtraSoftwareDeps: []string{"crosvm_no_gpu"},
			},
			{
				Name:              "gpu",
				Val:               "virgl",
				ExtraSoftwareDeps: []string{"crosvm_gpu"},
			}}),
	})
}

func GPUEnabled(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	expectedDevice := s.Param().(string)
	defer crostini.RunCrostiniPostTest(ctx, cont)

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
