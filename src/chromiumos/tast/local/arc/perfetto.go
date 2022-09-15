// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"

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
		localTempResultPath   = localPerfettoTraceDir + "perfetto.trace"
	)

	config, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read config file")
	}

	shellCmd := a.Command(ctx, "perfetto", "-o", localTempResultPath, "--txt", "--config", "-")
	shellCmd.Cmd.Stdin = bytes.NewReader(config)

	if err := shellCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start perfetto trace")
	}
	defer shellCmd.Wait()

	ferr := f(ctx)

	// If earlyExit, stop tracing immediately. Or wait tracing finish.
	if earlyExit {
		shellCmd.Kill()
	} else {
		shellCmd.Wait(testexec.DumpLogOnError)
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
