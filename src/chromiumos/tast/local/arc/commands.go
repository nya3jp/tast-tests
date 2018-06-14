// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"os/exec"
	"strings"
)

// Command runs a command in Android container with adb shell.
func Command(name string, arg ...string) *exec.Cmd {
	// Unfortunately, adb shell always passes the command line to /bin/sh, so
	// we need to escape arguments.
	escaped := make([]string, len(arg)+1)
	escaped[0] = shellEscape(name)
	for i, a := range arg {
		escaped[i+1] = shellEscape(a)
	}
	shell := "exec " + strings.Join(escaped, " ")
	return adbCommand("shell", shell)
}

func shellEscape(s string) string {
	return "'" + strings.Replace(s, "'", `'"'"'`, -1) + "'"
}

// bootstrapCommand runs a command with android-sh.
// Command execution environment of android-sh is not exactly the same as actual
// Android container, so this should be used only before ADB connection gets
// ready.
func bootstrapCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command("android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}

// SendIntent sends an intent with "am start" command.
func SendIntent(action, data string) error {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return Command("am", args...).Run()
}
