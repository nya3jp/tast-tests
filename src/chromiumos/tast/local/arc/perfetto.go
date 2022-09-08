// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// ForceEnableTrace will overwrite tracing_on flag in kernel tracefs. In some cases this flag
// are occupied by unknown reason during ARC booting. Failure may caused by permission issue or
// wrong debugfs path.
func (a *ARC) ForceEnableTrace(ctx context.Context) error {
	const (
		cmd = "echo 0 > "
		// TODO(sstan): Figure out different tracePath for R/T and x86/arm.
		// tracePath is the path of switcher in debugfs. The path may depend on the kernel version.
		tracePath = "/sys/kernel/tracing/tracing_on"
	)
	a.device.Root(ctx)
	if err := a.device.Command(ctx, "shell", cmd+tracePath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to force enable trace")
	}
	return nil
}

// RunPerfettoTrace will push the config from configTxtPath to ARC device, run the perfetto basing on config,
// and pull the trace result from ARC device to traceResultPath.
func (a *ARC) RunPerfettoTrace(ctx context.Context, traceConfigTxtPath, traceResultPath string) error {
	const (
		perfettoTraceDir     = "/data/misc/perfetto-traces/"
		localConfigPath      = perfettoTraceDir + "config"
		localTraceResultPath = perfettoTraceDir + "perfetto.trace"
	)
	// TODO(sstan): Currently access |perfettoTraceDir| in ARC require root permission. Need find a way to
	// access it in general permission so that we can run it in all of build target.
	a.device.Root(ctx)
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
