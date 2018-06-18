// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
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
// This function is called by chrome.New, so all functions in this package
// assume this function has already been called.
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
	cmd := bootstrapCommand(ctx, "logcat", "-b", "events", "*:S", "arc_system_event")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed creating stdout pipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	err = func() error {
		defer cmd.Wait()
		defer cmd.Kill()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasSuffix(line, " "+name) {
				return nil
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}
		return errors.New("EOF reached (maybe logcat crashed?)")
	}()

	if err != nil {
		cmd.DumpLog(ctx)
	}
	return err
}

// waitProp waits for Android prop name is set to value.
func waitProp(ctx context.Context, name, value string) error {
	for {
		loop := `while [ "$(getprop "$1")" != "$2" ]; do sleep 0.1; done`
		cmd := bootstrapCommand(ctx, "sh", "-c", loop, "-", name, value)
		if err := cmd.Run(); err == nil {
			return nil
		}

		// android-sh failed, implying Android container is not up yet.
		time.Sleep(time.Second)
	}
}
