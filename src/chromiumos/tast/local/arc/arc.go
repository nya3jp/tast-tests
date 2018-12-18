// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arc supports interacting with the ARC framework, which is used to run Android applications on Chrome OS.
package arc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// BootTimeout is the maximum amount of time allotted for ARC to boot.
	// Tests that call New should declare a timeout that's at least this long.
	BootTimeout = 120 * time.Second

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

// New waits for Android to finish booting.
//
// ARC must be enabled in advance by passing chrome.ARCEnabled to chrome.New.
//
// After this function returns successfully, you can assume BOOT_COMPLETED
// intent has been broadcast from Android system, and ADB connection is ready.
// Note that this does not necessarily mean all ARC mojo services are up; call
// WaitIntentHelper() to wait for ArcIntentHelper to be ready, for example.
//
// Returned ARC instance must be closed when the test is finished.
func New(ctx context.Context, outDir string) (*ARC, error) {
	bctx, cancel := context.WithTimeout(ctx, BootTimeout)
	defer cancel()

	if err := ensureARCEnabled(); err != nil {
		return nil, err
	}

	testing.ContextLog(bctx, "Waiting for Android boot")

	// service.adb.tcp.port is set by Android init very early in boot process.
	// Wait for it to ensure Android container is there.
	const androidInitProp = "service.adb.tcp.port"
	if err := waitProp(bctx, androidInitProp, "5555"); err != nil {
		return nil, errors.Wrapf(err, "Android failed to boot in very early stage: %s not set", androidInitProp)
	}

	// At this point we can start logcat.
	logcatPath := filepath.Join(outDir, logcatName)
	cmd, err := startLogcat(ctx, logcatPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start logcat")
	}
	defer func() {
		if cmd != nil {
			cmd.Kill()
			cmd.Wait()
		}
	}()

	// This property is set by the Android system server just before LOCKED_BOOT_COMPLETED is broadcast.
	const androidBootProp = "sys.boot_completed"
	if err := waitProp(bctx, androidBootProp, "1"); err != nil {
		return nil, diagnose(logcatPath, errors.Wrapf(err, "%s not set", androidBootProp))
	}

	// Android container is up. Set up ADB auth in parallel to Android boot since
	// ADB local server takes a few seconds to start up.
	ch := make(chan error, 1)
	go func() {
		ch <- setUpADBAuth(ctx)
	}()

	// This property is set by ArcAppLauncher when it receives BOOT_COMPLETED.
	const arcBootProp = "ro.arc.boot_completed"
	if err := waitProp(bctx, arcBootProp, "1"); err != nil {
		return nil, diagnose(logcatPath, errors.Wrapf(err, "%s not set", arcBootProp))
	}

	// Android has booted. Connect to ADB.
	if err := <-ch; err != nil {
		return nil, diagnose(logcatPath, errors.Wrap(err, "failed setting up ADB auth"))
	}
	if err := connectADB(ctx); err != nil {
		return nil, diagnose(logcatPath, errors.Wrap(err, "failed connecting to ADB"))
	}

	arc := &ARC{cmd}
	cmd = nil
	return arc, nil
}

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func (a *ARC) WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for ArcIntentHelper")
	const prop = "ro.arc.intent_helper.ready"
	if err := waitProp(ctx, prop, "1"); err != nil {
		return errors.Wrapf(err, "property %s not set", prop)
	}
	return nil
}

// ensureARCEnabled makes sure ARC is enabled by a command line flag to Chrome.
func ensureARCEnabled() error {
	args, err := getChromeArgs()
	if err != nil {
		return errors.Wrap(err, "failed getting Chrome args")
	}

	for _, a := range args {
		if a == "--arc-start-mode=always-start" || a == "--arc-start-mode=always-start-with-no-play-store" {
			return nil
		}
	}
	return errors.New("ARC is not enabled; pass chrome.ARCEnabled to chrome.New")
}

// getChromeArgs returns command line arguments of the Chrome browser process.
func getChromeArgs() ([]string, error) {
	pid, err := chrome.GetRootPID()
	if err != nil {
		return nil, err
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return proc.CmdlineSlice()
}

// startLogcat starts a logcat command saving Android logs to path.
func startLogcat(ctx context.Context, path string) (*testexec.Cmd, error) {
	cmd := bootstrapCommand(ctx, "logcat")
	f, err := os.Create(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create logcat file")
	}
	defer f.Close()
	cmd.Stdout = f
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// waitProp waits for Android prop name is set to value.
func waitProp(ctx context.Context, name, value string) error {
	loop := `while [ "$(getprop "$1")" != "$2" ]; do sleep 0.1; done`
	return testing.Poll(ctx, func(ctx context.Context) error {
		return bootstrapCommand(ctx, "sh", "-c", loop, "-", name, value).Run()
	}, &testing.PollOptions{Interval: time.Second})
}
