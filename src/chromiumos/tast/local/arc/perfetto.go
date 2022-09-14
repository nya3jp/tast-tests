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

// PerfettoTrace will push the config from traceConfigPath to ARC device, start the perfetto
// basing on config, run the function, and pull the trace result from ARC device to
// traceResultPath. Note that if earlyExit is true, the perfetto tracing will be stopped
// after test function return.
func (a *ARC) PerfettoTrace(ctx context.Context, traceConfigPath, traceResultPath string, earlyExit bool, f func(context.Context) error) error {
	// Perfetto related path inner ARC.
	const (
		localPerfettoTraceDir = "/data/misc/perfetto-traces/"
		localTempConfigPath   = localPerfettoTraceDir + "config"
		localTempResultPath   = localPerfettoTraceDir + "perfetto.trace"
	)

	// Currently ARC shell does not have write permission in |localPerfettoTraceDir|. So here use root permission as
	// a workaround. Note that it may not work some ARC builds.
	// TODO(sstan): Need change to use standin to pass config rather than create a config file.
	a.device.Root(ctx)

	if err := a.PushFile(ctx, traceConfigPath, localTempConfigPath); err != nil {
		return errors.Wrapf(err, "failed to push perfetto config file from %v to ARC path %v", traceConfigPath, localTempResultPath)
	}

	cmd := a.Command(ctx, "perfetto", "--txt", "--config", localTempConfigPath, "-o", localTempResultPath)

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start perfetto trace")
	}
	defer cmd.Wait()

	ferr := f(ctx)

	// If earlyExit, stop tracing immediately. Or wait tracing finish.
	if earlyExit {
		cmd.Kill()
	} else {
		cmd.Wait(testexec.DumpLogOnError)
	}

	// Pull trace result whatever test function succeeded or failed.
	if err := a.PullFile(ctx, localTempResultPath, traceResultPath); err != nil {
		return errors.Wrapf(err, "failed to pull perfetto from ARC path %v to %v", localTempResultPath, traceResultPath)
	}

	if ferr != nil {
		return errors.Wrap(ferr, "finish trace but errors happen on test func")
	}

	return nil
}
