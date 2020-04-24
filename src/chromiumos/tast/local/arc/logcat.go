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

// WaitForExpInLogcat waits for a regexp to appear in the logcat with timeout.
func WaitForExpInLogcat(ctx context.Context, a *ARC, exp *regexp.Regexp, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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

	done := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			l := scanner.Text()
			if exp.MatchString(l) {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "cannot match regexp %s in logcat before timeout(%s)", exp, timeout)
	}
}
