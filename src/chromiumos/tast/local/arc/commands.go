// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import "os/exec"

// Command runs a command in Android container with android-sh.
func Command(name string, arg ...string) *exec.Cmd {
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
