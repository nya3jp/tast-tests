// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"strings"

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

// BroadcastResult is the parsed result of an Android Activity Manager broadcast.
type BroadcastResult struct {
	// The result value of the broadcast.
	result int
	// Optional: Additional data to be passed with the result.
	data *string
	// Optional: A bundle of extra data passed with the result.
	// TODO(springerm): extras is a key-value map and should be parsed.
	extras *string
}

const (
	// Activity.RESULT_OK
	intentResultActivityResultOk = -1
)

// BroadcastIntent broadcasts an intent with "am broadcast" and returns the result.
func (a *ARC) BroadcastIntent(ctx context.Context, action string, params ...string) (*BroadcastResult, error) {
	args := []string{"broadcast", "-a", action}
	args = append(args, params...)

	output, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	// TODO(springerm): Find a way to report metrics from Android apps, so we can avoid this parsing hackery.
	// broadcastResultRegexp matches the result from an Android Activity Manager broadcast.
	broadcastResultRegexp := regexp.MustCompile(`Broadcast completed: result=(-?[0-9]+)(, data="((\\.|[^\\"])*)")?(, extras: Bundle\[(.*)\])?`)
	m := broadcastResultRegexp.FindSubmatch(output)

	if m == nil {
		return nil, errors.Errorf("unable to parse broadcast result for %s: %q", action, output)
	}

	resultValue, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return nil, errors.Errorf("unable to parse broadcast result value for %s: %q", action, string(m[1]))
	}

	broadcastResult := BroadcastResult{}
	broadcastResult.result = resultValue

	// `m[3]` matches the data value. `m[2]` matches the entire "data=\"...\"" part.
	// We have to check `m[2]` because the data could be an empty string, which is different from "no data", in which case we return nil.
	data := string(m[3])
	if string(m[2]) != "" {
		broadcastResult.data = &data
	}

	extras := string(m[6])
	if string(m[5]) != "" {
		broadcastResult.extras = &extras
	}

	return &broadcastResult, nil
}

// BroadcastIntentGetData broadcasts an intent with "am broadcast" and returns the result data.
func (a *ARC) BroadcastIntentGetData(ctx context.Context, action string, params ...string) (string, error) {
	result, err := a.BroadcastIntent(ctx, action, params...)
	if err != nil {
		return "", err
	}

	if result.result != intentResultActivityResultOk {
		if result.data == nil {
			return "", errors.Errorf("broadcast of %q failed, status = %d", action, result.result)
		}

		return "", errors.Errorf("broadcast of %q failed, status = %d, data = %q", action, result.result, *result.data)
	}

	if result.data == nil {
		return "", errors.Errorf("broadcast of %q has no result data", action)
	}

	return *result.data, nil
}
