// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// StartLogCollectors starts log collectors.
func StartLogCollectors(ctx context.Context, host *ssh.Conn, logCollectors map[string]*log.Collector, logsToCollect []string) error {
	for _, p := range logsToCollect {
		logger, err := log.StartCollector(ctx, host, p)
		if err != nil {
			return errors.Wrap(err, "failed to start log collector")
		}
		logCollectors[p] = logger
	}
	return nil
}

// StopLogCollectors closes all log collectors spawned.
func StopLogCollectors(ctx context.Context, logCollectors map[string]*log.Collector) error {
	var firstErr error
	for _, c := range logCollectors {
		if err := c.Close(); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, err)
		}
	}
	return firstErr
}

// CollectLogs downloads log files from router to $OutDir/debug/$r.Name with suffix
// appended to the filenames.
func CollectLogs(ctx context.Context, r support.Router, logCollectors map[string]*log.Collector, logsToCollect []string, suffix string) error {
	ctx, st := timing.Start(ctx, "collectLogs")
	defer st.End()

	baseDir := filepath.Join("debug", r.RouterName())

	for _, src := range logsToCollect {
		dst := filepath.Join(baseDir, filepath.Base(src)+suffix)
		collector := logCollectors[src]
		if collector == nil {
			testing.ContextLogf(ctx, "No log collector for %s found", src)
			continue
		}
		f, err := fileutil.PrepareOutDirFile(ctx, dst)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to collect %q, err: %v", src, err)
			continue
		}
		if err := collector.Dump(f); err != nil {
			testing.ContextLogf(ctx, "Failed to dump %q logs, err: %v", src, err)
			continue
		}
		absDstPath, err := filepath.Abs(f.Name())
		if err != nil {
			testing.ContextLogf(ctx, "Failed to get absolute file path of destination log file %q, err: %v", dst, err)
			continue
		}
		testing.ContextLogf(ctx, "Dumped captured router logs from %q to local chroot file %q", src, absDstPath)
	}
	return nil
}
