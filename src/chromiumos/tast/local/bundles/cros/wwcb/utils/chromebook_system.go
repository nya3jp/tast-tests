// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// SuspendAndResume calls powerd_dbus_suspend command to suspend the system and lets it
// stay sleep for the given duration and then wake up.
func SuspendAndResume(ctx context.Context, cr *chrome.Chrome, sleepTime int) error {
	// The actual time used to suspend and weekup the system is:
	// 		(time to suspend the system) + (sleep time) + (time to wakeup the system)
	// Tast runner might time out if DUT is inaccessible for more than 60 seconds.
	// We allow 30-second maximum sleep time, trying to keep the total suspend/wakeup time
	// under 1 minute.
	const maxSleepTime = 30
	if sleepTime > maxSleepTime {
		return errors.Errorf("suspend time should less than %d seconds", maxSleepTime)
	}

	// timeout, according to powerd_dbus_suspend help page, defines how long to wait for
	// a resume signal in seconds. We add 20 seconds to maxSleepTime to ensure the command
	// will exit if the whole suspend/wakeup procedure couldn't trigger a resume signal for
	// any reason within this time.
	timeout := maxSleepTime + 20

	// Read wakeup count here to prevent suspend retries, which happens without user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		return errors.Wrap(err, "failed to read wakeup count before suspend")
	}

	cmd := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		"--disable_dark_resume=true",
		fmt.Sprintf("--timeout=%d", timeout),
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		fmt.Sprintf("--suspend_for_sec=%d", sleepTime),
	)
	testing.ContextLogf(ctx, "Start a DUT suspend of %d seconds: %s", sleepTime, cmd.Args)

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "powerd_dbus_suspend failed to properly suspend")
	}

	testing.ContextLog(ctx, "DUT resumes from suspend")
	return cr.Reconnect(ctx)
}

// WaitForFileSaved waits for the presence of the captured file with file name matching the specified
// change timeout from 5s to 60s
// refer to cca.go in pacakge cca
// pattern, size larger than zero, and modified time after the specified timestamp.
func WaitForFileSaved(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	const timeout = time.Minute
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read the directory")
		}
		for _, file := range files {
			if file.Size() == 0 || file.ModTime().Before(ts) {
				continue
			}
			if _, ok := seen[file.Name()]; ok {
				continue
			}
			seen[file.Name()] = struct{}{}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				result = file
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrapf(err, "no matching output file found after %v", timeout)
	}
	return result, nil
}

// PrettyPrint pretty print objects.
func PrettyPrint(ctx context.Context, i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	testing.ContextLog(ctx, string(s))

}

// RunOrFatal runOrFatal runs body as subtest, then invokes s.Fatal if it returns an error
func RunOrFatal(ctx context.Context, s *testing.State, name string, body func(context.Context, *testing.State) error) bool {
	return s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		if err := body(ctx, s); err != nil {
			s.Fatal("subtest failed: ", err)
		}
	})
}
