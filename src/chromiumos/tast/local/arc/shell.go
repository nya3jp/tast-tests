// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os/exec"
)

// Command runs a command in Android container.
func Command(name string, arg ...string) *exec.Cmd {
	return exec.Command("android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}

// CommandContext runs a command in Android container.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "android-sh", append([]string{"-c", "exec \"$@\"", "-", name}, arg...)...)
}
