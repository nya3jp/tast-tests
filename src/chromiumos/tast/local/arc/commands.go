// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// Command runs a command in Android container via adb.
func (a *ARC) Command(ctx context.Context, name string, args ...string) *testexec.Cmd {
	// adb shell executes the command via /bin/sh, so here it is necessary
	// to escape.
	cmd := "exec " + shutil.EscapeSlice(append([]string{name}, args...))
	return adbCommand(ctx, "shell", cmd)
}

// BootstrapCommand runs a command with android-sh.
//
// It is very rare you want to call this function from your test; call Command
// instead. A valid use case would to run commands in the Android mini
// container, to set up adb, etc.
//
// This function should be called only after WaitAndroidInit returns
// successfully. Please keep in mind that command execution environment of
// android-sh is not exactly the same as the actual Android container.
func BootstrapCommand(ctx context.Context, name string, arg ...string) *testexec.Cmd {
	// Refuse to find an executable with $PATH.
	// android-sh inserts /vendor/bin before /system/bin in $PATH, and /vendor/bin
	// contains very similar executables as /system/bin on some boards (e.g. nocturne).
	// In particular, /vendor/bin/sh is rarely what you want since it drops
	// /system/bin from $PATH. To avoid such mistakes, refuse to run executables
	// without explicitly specifying absolute paths. To run shell commands,
	// specify /system/bin/sh.
	// See: http://crbug.com/949853
	if !strings.HasPrefix(name, "/") {
		panic("Refusing to search $PATH; specify an absolute path instead")
	}
	return testexec.CommandContext(ctx, "android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}

// SendIntentCommand returns a Cmd to send an intent with "am start" command.
func (a *ARC) SendIntentCommand(ctx context.Context, action, data string) *testexec.Cmd {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return a.Command(ctx, "am", args...)
}

// GetProp returns the Android system property indicated by the specified key.
func (a *ARC) GetProp(ctx context.Context, key string) (string, error) {
	o, err := a.Command(ctx, "getprop", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(o)), nil
}

// WaitForExpInLogcat waits for a regexp to appear in the logcat with timeout.
func (a *ARC) WaitForExpInLogcat(ctx context.Context, exp *regexp.Regexp, timeout time.Duration) error {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	cmd := a.Command(ctx, "logcat")

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to open StdoutPipe")
	}
	defer pipe.Close()

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start %s", shutil.EscapeSlice(cmd.Args))
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	done := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			l := scanner.Text()
			if exp.MatchString(l) {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "cannot match regexp %s in logcat before timeout(%s)", exp, timeout)
	}
}
