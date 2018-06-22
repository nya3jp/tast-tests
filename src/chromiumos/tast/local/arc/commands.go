// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/testexec"
)

// Command runs a command in Android container via adb.
//
// Be aware of many restrictions of adb: return code is always 0, stdin is not
// connected, and stderr is mixed to stdout.
func Command(ctx context.Context, name string, arg ...string) *testexec.Cmd {
	// adb exec-out is like adb shell, but skips CR/LF conversion.
	// Unfortunately, adb exec-out always passes the command line to /bin/sh, so
	// we need to escape arguments.
	shell := "exec " + testexec.ShellEscapeArray(append([]string{name}, arg...))
	return adbCommand(ctx, "exec-out", shell)
}

// bootstrapCommand runs a command with android-sh.
// Command execution environment of android-sh is not exactly the same as actual
// Android container, so this should be used only before ADB connection gets
// ready.
func bootstrapCommand(ctx context.Context, name string, arg ...string) *testexec.Cmd {
	return testexec.CommandContext(ctx, "android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}

// SendIntentCommand returns a Cmd to send an intent with "am start" command.
func SendIntentCommand(ctx context.Context, action, data string) *testexec.Cmd {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return Command(ctx, "am", args...)
}
