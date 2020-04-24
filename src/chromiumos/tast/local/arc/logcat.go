// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

// RegexpPred returns a function to be passed to WaitForLogcat that returns true if a given regexp is matched in that line.
func RegexpPred(exp *regexp.Regexp) func(string) bool {
	return func(l string) bool {
		return exp.MatchString(l)
	}
}

// WaitForLogcat keeps scanning logcat with a timeout. The function pred is called on the logcat contents line by line. This function returns successfully if pred returns true. If pred never returns true, this function returns an error.
func WaitForLogcat(ctx context.Context, a *ARC, pred func(string) bool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := a.Command(ctx, "logcat")

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to open StdoutPipe")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start %s", shutil.EscapeSlice(cmd.Args))
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	status := make(chan error)

	go func() {
		// pred might panic, so always write something to the channel.
		defer func() {
			status <- errors.New("could not match pred")
		}()
		defer pipe.Close()

		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			if pred(scanner.Text()) {
				status <- nil
				return
			}
		}

		status <- errors.New("reached EOF of logcat, but pred has not matched")
	}()

	select {
	case err := <-status:
		if err != nil {
			return errors.Wrap(err, "error while scanning logcat")
		}
		return nil
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "could not match pred in logcat before timeout(%s)", timeout)
	}
}
