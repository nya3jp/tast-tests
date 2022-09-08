// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"context"
)

// ForceEnableTrace will overwrite tracing_on flag in kernel tracefs. Sometimes this flag
// occupied by unknown reason during ARC booting. Failure may caused by permission issue or
// wrong debugfs path.
func (a *ARC) ForceEnableTrace(ctx context.Context) error {
	a.device.Root(ctx)
	const (
		cmd = "echo 0 > "
		// tracePath is the path of switcher in debugfs. The path may depend on the kernel version.
		tracePath = "/sys/kernel/tracing/tracing_on"
	)

	if err := a.device.Command(ctx, "shell", cmd+tracePath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to force enable trace")
	}
	return nil
}

// RunPerfettoTrace will push the config from configTxtPath to ARC device, run the perfetto basing on config,
// and pull the trace result from ARC device to traceResultPath.
func (a *ARC) RunPerfettoTrace(ctx context.Context, traceConfigTxtPath string, traceResultPath string) error {
	// a.Device().Root(ctx)
	const (
		perfettoTraceDir     = "/data/misc/perfetto-traces/"
		localConfigPath      = perfettoTraceDir + "config"
		localTraceResultPath = perfettoTraceDir + "perfetto.trace"
	)
	if err := a.PushFile(ctx, traceConfigTxtPath, localConfigPath); err != nil {
		return errors.Wrapf(err, "failed to push perfetto config file from %v to %v", traceConfigTxtPath, localConfigPath)
	}

	if err := a.Command(ctx, "perfetto", "--txt", "--config", localConfigPath, "-o", localTraceResultPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run perfetto")
	}

	if err := a.PullFile(ctx, localTraceResultPath, traceResultPath); err != nil {
		return errors.Wrapf(err, "failed to pull perfetto from %v to %v", localTraceResultPath, traceResultPath)
	}
	return nil
}
