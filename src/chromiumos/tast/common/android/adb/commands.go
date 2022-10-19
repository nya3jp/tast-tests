// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

// ShellCommand returns a command in Android shell via adb.
func (d *Device) ShellCommand(ctx context.Context, name string, args ...string) *testexec.Cmd {
	// adb shell executes the command via /bin/sh, so here it is necessary
	// to escape.
	cmd := "exec " + shutil.EscapeSlice(append([]string{name}, args...))
	return d.Command(ctx, "shell", cmd)
}

// SendIntentCommand returns a Cmd to send an intent with "am start" command.
func (d *Device) SendIntentCommand(ctx context.Context, action, data string) *testexec.Cmd {
	args := []string{"start", "-a", action}
	if len(data) > 0 {
		args = append(args, "-d", data)
	}
	return d.ShellCommand(ctx, "am", args...)
}

// GetProp returns the Android system property indicated by the specified key.
func (d *Device) GetProp(ctx context.Context, key string) (string, error) {
	o, err := d.ShellCommand(ctx, "getprop", key).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(o)), nil
}

// BroadcastResult is the parsed result of an Android Activity Manager broadcast.
type BroadcastResult struct {
	// The result value of the broadcast.
	Result int
	// Optional: Additional data to be passed with the result.
	Data *string
	// Optional: A bundle of extra data passed with the result.
	// TODO(springerm): extras is a key-value map and should be parsed.
	Extras *string
}

const (
	// Activity.RESULT_OK
	intentResultActivityResultOk = -1
)

// BroadcastIntent broadcasts an intent with "am broadcast" and returns the result.
func (d *Device) BroadcastIntent(ctx context.Context, action string, params ...string) (*BroadcastResult, error) {
	args := []string{"broadcast", "-a", action}
	args = append(args, params...)

	output, err := d.ShellCommand(ctx, "am", args...).Output(testexec.DumpLogOnError)
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
	broadcastResult.Result = resultValue

	// `m[3]` matches the data value. `m[2]` matches the entire "data=\"...\"" part.
	// We have to check `m[2]` because the data could be an empty string, which is different from "no data", in which case we return nil.
	data := string(m[3])
	if string(m[2]) != "" {
		broadcastResult.Data = &data
	}

	extras := string(m[6])
	if string(m[5]) != "" {
		broadcastResult.Extras = &extras
	}

	return &broadcastResult, nil
}

// BroadcastIntentGetData broadcasts an intent with "am broadcast" and returns the result data.
func (d *Device) BroadcastIntentGetData(ctx context.Context, action string, params ...string) (string, error) {
	result, err := d.BroadcastIntent(ctx, action, params...)
	if err != nil {
		return "", err
	}

	if result.Result != intentResultActivityResultOk {
		if result.Data == nil {
			return "", errors.Errorf("broadcast of %q failed, status = %d", action, result.Result)
		}

		return "", errors.Errorf("broadcast of %q failed, status = %d, data = %q", action, result.Result, *result.Data)
	}

	if result.Data == nil {
		return "", errors.Errorf("broadcast of %q has no result data", action)
	}

	return *result.Data, nil
}

// BugReport returns bugreport of the device.
func (d *Device) BugReport(ctx context.Context, path string) error {
	return d.Command(ctx, "bugreport", path).Run(testexec.DumpLogOnError)
}
