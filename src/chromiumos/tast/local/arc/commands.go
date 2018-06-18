// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/exec"
)

// Command runs a command in Android container with adb shell.
func Command(ctx context.Context, name string, arg ...string) *exec.Cmd {
	// Unfortunately, adb shell always passes the command line to /bin/sh, so
	// we need to escape arguments.
	shell := "exec " + exec.ShellEscapeArray(append([]string{name}, arg...))
	return adbCommand(ctx, "shell", shell)
}

// bootstrapCommand runs a command with android-sh.
// Command execution environment of android-sh is not exactly the same as actual
// Android container, so this should be used only before ADB connection gets
// ready.
func bootstrapCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}

// SendIntentCommand returns a Cmd to send an intent with "am start" command.
func SendIntentCommand(ctx context.Context, action, data string) *exec.Cmd {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return Command(ctx, "am", args...)
}
