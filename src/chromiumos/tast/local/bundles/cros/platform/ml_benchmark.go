// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/platform/ml"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmark,
		Desc: "Verifies that the ML Benchmarks work end to end",
		Contacts: []string{
			"franklinh@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		SoftwareDeps: []string{"ml_benchmark"},
	})
}

func MLBenchmark(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--workspace_path=/usr/local/ml_benchmark")
	var stderrBytes bytes.Buffer
	var stdoutBytes bytes.Buffer
	cmd.Stderr = &stderrBytes
	cmd.Stdout = &stdoutBytes

	if err := cmd.Run(); err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	stdout := stdoutBytes.String()
	stderr := stderrBytes.String()

	logFilename := "ml_benchmark_log.txt"

	ml.LogOutput(s, logFilename, shutil.EscapeSlice(cmd.Args), stdout, stderr)

	if strings.Contains(stdout, "ERROR") ||
		strings.Contains(stderr, "ERROR") ||
		strings.Contains(stderr, "FATAL") {
		s.Errorf("%s contained output with an error. See %s",
			shutil.EscapeSlice(cmd.Args),
			logFilename)
	}
}
