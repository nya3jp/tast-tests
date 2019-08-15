// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gpuenabled provides a test to place expectations on whether the GPU
// is enabled for different crostini startup configurations
package gpuenabled

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RunTest runs a simple test to ensure the command line works.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container, expectedDevice string) {
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
