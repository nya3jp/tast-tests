// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// Command runs a command in Android container via adb.
func (a *ARC) Command(ctx context.Context, name string, arg ...string) *testexec.Cmd {
	// adb exec-out is like adb shell, but skips CR/LF conversion.
	// Unfortunately, adb exec-out always passes the command line to /bin/sh, so
	// we need to escape arguments.
	shell := "exec " + shutil.EscapeSlice(append([]string{name}, arg...))
	return adbCommand(ctx, "exec-out", shell)
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
