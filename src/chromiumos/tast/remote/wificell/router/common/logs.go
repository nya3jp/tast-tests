// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// StartLogCollectors starts log collectors with log.StartCollector.
func StartLogCollectors(ctx context.Context, host *ssh.Conn, logsToCollect []string, tailFollowNameSupported bool) (map[string]*log.Collector, error) {
	logCollectors := make(map[string]*log.Collector)
	for _, p := range logsToCollect {
		logger, err := log.StartCollector(ctx, host, p, tailFollowNameSupported)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start log collector")
		}
		logCollectors[p] = logger
	}
	return logCollectors, nil
}

// StopLogCollectors closes all log collectors spawned.
func StopLogCollectors(ctx context.Context, logCollectors map[string]*log.Collector) error {
	var firstErr error
	for _, c := range logCollectors {
		if err := c.Close(); err != nil {
			utils.CollectFirstErr(ctx, &firstErr, err)
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

	var firstErr error
	for _, src := range logsToCollect {
		dst := filepath.Join(baseDir, filepath.Base(src)+suffix)
		collector := logCollectors[src]
		if collector == nil {
			testing.ContextLogf(ctx, "No log collector for %s found", src)
			utils.CollectFirstErr(ctx, &firstErr, errors.Errorf("failed to find log collector %q", src))
			continue
		}
		f, err := fileutil.PrepareOutDirFile(ctx, dst)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to collect %q, err: %v", src, err)
			utils.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to collect %q", src))
			continue
		}
		if err := collector.Dump(f); err != nil {
			testing.ContextLogf(ctx, "Failed to dump %q logs, err: %v", src, err)
			utils.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to dump %q logs", src))
		}
	}
	return firstErr
}

// CollectSyslogdLogs writes the collected syslogd logs to a log file. An
// optional logName may be specified.
func CollectSyslogdLogs(ctx context.Context, r support.Router, logCollector *log.SyslogdCollector, logName string) error {
	ctx, st := timing.Start(ctx, "collectLogs")
	defer st.End()
	// Prepare output file.
	dstLogFilename := buildLogFilename("syslogd", logName)
	dstFilePath := filepath.Join("debug", r.RouterName(), dstLogFilename)
	f, err := fileutil.PrepareOutDirFile(ctx, dstFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare output dir file %q", dstFilePath)
	}
	// Dump buffer of collected logs to file.
	if err := logCollector.Dump(f); err != nil {
		return errors.Wrapf(err, "failed to dump syslogd logs to %q", dstFilePath)
	}
	return nil
}

// buildLogFilename builds a log filename with a minimal timestamp prefix, all
// the name parts in the middle delimited by "_" with non-word characters
// replaced with underscores, and a ".log" file extension.
//
// Example result: "20220523-122753_syslogd_pre_setup"
func buildLogFilename(nameParts ...string) string {
	// Build timestamp prefix.
	timestamp := time.Now().Format(TimestampFileFormat)
	// Join and sanitize name parts.
	name := strings.Join(nameParts, "_")
	name = regexp.MustCompile("\\W").ReplaceAllString(name, "_")
	name = regexp.MustCompile("_+").ReplaceAllString(name, "_")
	// Combine timestamp, name, and extension.
	return fmt.Sprintf("%s_%s.log", timestamp, name)
}
