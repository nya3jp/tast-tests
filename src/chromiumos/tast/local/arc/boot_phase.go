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

const (
	bootTimeout         = 60 * time.Second
	intentHelperTimeout = 20 * time.Second
)

// WaitBootCompleted waits for Android to finish booting. After this function
// returns successfully, you can assume BOOT_COMPLETED intent has been broadcast
// from Android system. Note that this does not necessarily mean all ARC mojo
// services are up; call WaitIntentHelper() to wait for ArcIntentHelper to be
// ready, for example.
//
// This function is called from chrome package, so all functions in this
// package assume this function has already been called.
func WaitBootCompleted(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, bootTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for Android boot")

	// sys.boot_completed is set by Android system server just before
	// LOCKED_BOOT_COMPLETED is broadcast.
	if err := waitProp(ctx, "sys.boot_completed", "1"); err != nil {
		return err
	}

	// Wait for BOOT_COMPLETED to be observed by ArcAppLauncher.
	return waitSystemEvent(ctx, "ArcAppLauncher:started")
}

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for ArcIntentHelper")
	return waitSystemEvent(ctx, "ArcIntentHelperService:ready")
}

// waitSystemEvent blocks until logcat reports an ARC system event named name.
// An error is returned if logcat is failed or ctx's deadline is reached.
func waitSystemEvent(ctx context.Context, name string) error {
	cmd := bootstrapCommand("logcat", "-b", "events", "*:S", "arc_system_event")
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

	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
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

// waitProp waits for Android prop name is set to value.
func waitProp(ctx context.Context, name, value string) error {
	for {
		loop := `while [ "$(getprop "$1")" != "$2" ]; do sleep 0.1; done`
		cmd := bootstrapCommand("sh", "-c", loop, "-", name, value)
		// Enable Setpgid so we can terminate the whole subprocesses.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
			return err
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case err := <-done:
			if err == nil {
				return nil
			}
		case <-ctx.Done():
			// Negative PID means the process group led by the direct child process.
			// TODO(nya): It might not be safe to kill a process being waited.
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			return ctx.Err()
		}

		// android-sh failed, implying Android container is not up yet.
		time.Sleep(time.Second)
	}
}
