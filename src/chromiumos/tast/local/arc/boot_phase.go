// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	bootTimeout         = 60 * time.Second
	intentHelperTimeout = 20 * time.Second

	logcatName = "logcat.txt"
)

// ARC holds resources related to an active ARC session. Call Close to release
// those resources.
type ARC struct {
	logcat *testexec.Cmd // process saving Android logs.
}

// Close releases resources associated to ARC.
func (a *ARC) Close() error {
	a.logcat.Kill()
	return a.logcat.Wait()
}

// Start starts Android and waits to finish booting.
//
// After this function returns successfully, you can assume BOOT_COMPLETED
// intent has been broadcast from Android system, and ADB connection is ready.
// Note that this does not necessarily mean all ARC mojo services are up; call
// WaitIntentHelper() to wait for ArcIntentHelper to be ready, for example.
//
// Returned ARC instance must be closed when the test is finished.
//
// This function must be called at the start of all ARC tests. All functions in
// this package assumes this function has already been called.
func Start(ctx context.Context, c *chrome.Chrome, outDir string) (*ARC, error) {
	bctx, cancel := context.WithTimeout(ctx, bootTimeout)
	defer cancel()

	testing.ContextLog(bctx, "Enabling ARC")

	// Enable ARC. This will start the Android container.
	if err := enableARC(bctx, c); err != nil {
		return nil, fmt.Errorf("failed enabling ARC: %v", err)
	}

	testing.ContextLog(bctx, "Waiting for Android boot")

	// service.adb.tcp.port is set by Android init very early in boot process.
	// Wait for it to ensure Android container is there.
	if err := waitProp(bctx, "service.adb.tcp.port", "5555"); err != nil {
		return nil, fmt.Errorf("service.adb.tcp.port not set: %v", err)
	}

	// At this point we can start logcat.
	cmd, err := startLogcat(ctx, filepath.Join(outDir, logcatName))
	if err != nil {
		return nil, fmt.Errorf("failed to start logcat: %v", err)
	}
	defer func() {
		if cmd != nil {
			cmd.Kill()
			cmd.Wait()
		}
	}()

	// sys.boot_completed is set by Android system server just before
	// LOCKED_BOOT_COMPLETED is broadcast.
	if err := waitProp(bctx, "sys.boot_completed", "1"); err != nil {
		return nil, fmt.Errorf("LOCKED_BOOT_COMPLETED not observed: %v", err)
	}

	// ArcAppLauncher:started is emitted by ArcAppLauncher when it receives
	// BOOT_COMPLETED.
	if err := waitSystemEvent(bctx, "ArcAppLauncher:started"); err != nil {
		return nil, fmt.Errorf("BOOT_COMPLETED not observed: %v", err)
	}

	// Android has booted. Set up ADB.
	if err := SetUpADB(bctx); err != nil {
		return nil, fmt.Errorf("failed setting up ADB: %v", err)
	}

	arc := &ARC{cmd}
	cmd = nil
	return arc, nil
}

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for ArcIntentHelper")
	if err := waitSystemEvent(ctx, "ArcIntentHelperService:ready"); err != nil {
		return fmt.Errorf("waiting for ArcIntentHelperService:ready event: %v", err)
	}
	return nil
}

// enableARC enables ARC on the current session.
func enableARC(ctx context.Context, c *chrome.Chrome) error {
	conn, err := c.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	// TODO(derat): Consider adding more functionality (e.g. checking managed state)
	// from enable_play_store() in Autotest's client/common_lib/cros/arc_util.py.
	return conn.Exec(ctx, "chrome.autotestPrivate.setPlayStoreEnabled(true, function(enabled) {});")
}

// startLogcat starts a logcat command saving Android logs to path.
func startLogcat(ctx context.Context, path string) (*testexec.Cmd, error) {
	cmd := bootstrapCommand(ctx, "logcat")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create logcat file: %v", err)
	}
	cmd.Stdout = f
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
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

		if err := ctx.Err(); err != nil {
			return err
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return errors.New("logcat crashed")
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

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// android-sh failed, implying Android container is not up yet.
		time.Sleep(time.Second)
	}
}
