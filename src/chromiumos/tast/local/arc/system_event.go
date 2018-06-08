// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/testing"
)

const intentHelperTimeout = 60 * time.Second

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()
	return waitSystemEvent(ctx, "ArcIntentHelperService:ready")
}

// waitSystemEvent blocks until logcat reports an ARC system event named name.
// An error is returned if logcat is failed or ctx's deadline is reached.
func waitSystemEvent(ctx context.Context, name string) error {
	cmd := Command("logcat", "-b", "events", "*:S", "arc_system_event")
	// Enable Setpgid so we can terminate the whole subprocesses.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	// Negative PID means the process group led by the direct child process.
	defer syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	testing.ContextLogf(ctx, "Waiting for ARC system event %v", name)

	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			testing.ContextLog(ctx, line)
			if strings.HasSuffix(line, " "+name) {
				done <- nil
				return
			}
		}
		if err = scanner.Err(); err != nil {
			done <- err
		} else {
			done <- errors.New("EOF reached (maybe logcat crashed?)")
		}
	}()

	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
