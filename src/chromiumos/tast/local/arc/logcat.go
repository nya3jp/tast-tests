// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

// RegexpPred returns a function to be passed to WaitForLogcat that returns true if a given regexp is matched in that line.
func RegexpPred(exp *regexp.Regexp) func(string) bool {
	return func(l string) bool {
		return exp.MatchString(l)
	}
}

// WaitForLogcat keeps scanning logcat. The function pred is called on the logcat contents line by line. This function returns successfully if pred returns true. If pred never returns true, this function returns an error as soon as the context is done.
func (a *ARC) WaitForLogcat(ctx context.Context, pred func(string) bool) error {
	cmd := a.Command(ctx, "logcat")

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to open StdoutPipe")
	}
	defer pipe.Close()

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start %s", shutil.EscapeSlice(cmd.Args))
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	status := make(chan error, 2)

	go func() {
		// pred might panic, so always write something to the channel.
		defer func() {
			status <- errors.New("could not match pred")
		}()

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
		// This is usually a timeout.
		return errors.Wrap(ctx.Err(), "context was done while waiting match of pred in logcat")
	}
}
