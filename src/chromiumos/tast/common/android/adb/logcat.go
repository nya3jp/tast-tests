// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RegexpPred returns a function to be passed to WaitForLogcat that returns true if a given regexp is matched in that line.
func RegexpPred(exp *regexp.Regexp) func(string) bool {
	return func(l string) bool {
		return exp.MatchString(l)
	}
}

// WaitForLogcat keeps scanning logcat. The function pred is called on the logcat contents line by line. This function returns the candidate line successfully if pred returns true. If pred never returns true, this function returns an error as soon as the context is done.
// An optional quitFunc will be polled at regular interval, which can be used to break early if for example the activity which is supposed to print the exp has crashed
func (d *Device) WaitForLogcat(ctx context.Context, pred func(string) bool, quitFunc ...func() bool) (string, error) {
	if len(quitFunc) > 1 {
		return "", errors.New("only 1 quitFunc is supported")
	}

	cmd := d.ShellCommand(ctx, "logcat")

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", errors.Wrap(err, "failed to open StdoutPipe")
	}
	defer pipe.Close()

	if err := cmd.Start(); err != nil {
		return "", errors.Wrapf(err, "failed to start %s", shutil.EscapeSlice(cmd.Args))
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	status := make(chan error)
	breakChan := make(chan struct{})
	ret := ""

	go func() {
		// pred might panic, so always write something to the channel.
		defer func() {
			status <- errors.New("could not match pred")
		}()

		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			select {
			case <-breakChan:
				return
			default:
				if pred(scanner.Text()) {
					status <- nil
					ret = scanner.Text()
					return
				}
			}
		}

		status <- errors.New("reached EOF of logcat, but pred has not matched")
	}()

	if len(quitFunc) > 0 {
		go func() {
			testing.Poll(ctx, func(ctx context.Context) error {
				select {
				case <-breakChan:
					return nil
				default:
					if quitFunc[0]() {
						status <- nil
						return nil
					}
					return errors.New("still good")
				}
			}, &testing.PollOptions{})
		}()
	}

	select {
	case err := <-status:
		close(breakChan)
		if err != nil {
			return "", errors.Wrap(err, "error while scanning logcat")
		}
		return ret, nil
	case <-ctx.Done():
		// This is usually a timeout.
		close(breakChan)
		return "", errors.Wrap(ctx.Err(), "context was done while waiting match of pred in logcat")
	}
}

// ClearLogcat clears all logcat buffers.
func (d *Device) ClearLogcat(ctx context.Context) error {
	if err := d.ShellCommand(ctx, "logcat", "-b", "all", "-c").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to clear logcat logs")
	}
	return nil
}

// EnableVerboseLoggingForTag enables verbose logging for the specified tag.
func (d *Device) EnableVerboseLoggingForTag(ctx context.Context, tag string) error {
	if err := d.ShellCommand(ctx, "setprop", fmt.Sprintf("log.tag.%v", tag), "VERBOSE").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to enable verbose logging for tag %v", tag)
	}
	return nil
}

// DumpLogcat dumps logcat's output to the specified file.
func (d *Device) DumpLogcat(ctx context.Context, filePath string, opts ...string) error {
	out, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create logcat output file")
	}
	defer out.Close()

	params := append([]string{"logcat", "-d"}, opts...)
	cmd := d.Command(ctx, params...)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to dump logcat")
	}
	return nil
}

// OutputLogcatGrep greps logcat with the given string and returns the output.
// TODO(b/232537114): Change to a "d.ShellCommand" call after the underlying logic
// becomes usable.
func (d *Device) OutputLogcatGrep(ctx context.Context, grepArg string) ([]byte, error) {
	return d.Command(ctx, "shell", "logcat", "-d", "|", "grep", grepArg).Output()
}

// LogcatTimestamp is a logcat-formatted timestamp string:
// MM-DD hh:mm:ss.xxx ex: 06-15 17:03:00.887
type LogcatTimestamp string

// LogcatTimestampPattern is the regexp for matching a logcat timestamp string.
var LogcatTimestampPattern = regexp.MustCompile(`\d{1,2}-\d{1,2} \d{1,2}:\d{1,2}:\d{1,2}.\d{1,3}`)

// LatestLogcatTimestamp gets the timestamp of the latest logcat entry.
// This can be used as a marker to get logcat entries that only happen after
// this time, allowing for logcat dumps scoped to a particular test without
// needing to clear logcat's buffers.
func (d *Device) LatestLogcatTimestamp(ctx context.Context) (LogcatTimestamp, error) {
	out, err := d.Command(ctx, "logcat", "-t", "1").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get latest logcat entry")
	}
	timestamp := LogcatTimestampPattern.Find(out)
	return LogcatTimestamp(timestamp), nil
}

// DumpLogcatFromTimestamp dumps logcat's output to the specified file.
// The output will only contain entries that occurred after the timestamp.
func (d *Device) DumpLogcatFromTimestamp(ctx context.Context, filePath string, timestamp LogcatTimestamp) error {
	return d.DumpLogcat(ctx, filePath, "-T", fmt.Sprintf("%v", timestamp))
}
