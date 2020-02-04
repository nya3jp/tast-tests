// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash supports crash_reporter configuration.
package crash

import (
	"strings"

	"chromiumos/tast/errors"
)

// ReplaceCrashFilterString replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// By editing the core_pattern file (see manpage of core(5) for detail), the kernel
// is set up to call the crash reporter with the core dump as stdin when a process
// dies. This function adds a filter to the command line used to call the crash
// reporter. This is used to ignore crashes in which we have no interest.
func ReplaceCrashFilterString(oldPattern string, param string) (string, error) {
	if !strings.HasPrefix(oldPattern, "|") {
		return "", errors.Errorf("pattern should start with '|', but was: %s", oldPattern)
	}
	e := strings.Split(strings.TrimSpace(oldPattern), " ")
	var newargs []string
	replaced := false
	for _, s := range e {
		if !strings.HasPrefix(s, "--filter_in=") {
			newargs = append(newargs, s)
			continue
		}
		if len(param) == 0 {
			// Remove from list.
			continue
		}
		newargs = append(newargs, "--filter_in="+param)
		replaced = true
	}
	if len(param) != 0 && !replaced {
		newargs = append(newargs, "--filter_in="+param)
	}
	return strings.Join(newargs, " "), nil
}
