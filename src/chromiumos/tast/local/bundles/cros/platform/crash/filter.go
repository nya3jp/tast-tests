// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"
)

// replaceCrashFilterString replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// By editing the core_pattern file (see manpage of core(5) for detail), the kernel
// is set up to call the crash reporter with the core dump as stdin when a process
// dies. This function adds a filter to the command line used to call the crash
// reporter. This is used to ignore crashes in which we have no interest.
func replaceCrashFilterString(oldPattern, param string) (string, error) {
	if !strings.HasPrefix(oldPattern, "|") {
		return "", errors.Errorf("pattern should start with '|', but was: %s", oldPattern)
	}
	e := strings.Split(strings.TrimSpace(oldPattern), " ")
	newargs := make([]string, 0, len(e))
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

// replaceCrashFilterIn sets up the crash reporter to handle only the processes
// which has the specified name. If name is an empty, all crashes are handled.
// This is used to ignore crashes in which we have no interest.
// See ReplaceCrashFilterString in chromiumos/tast/common/crash/filter.go for more details.
func replaceCrashFilterIn(name string) error {
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	newPattern, err := replaceCrashFilterString(string(b), name)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(CorePattern, []byte(newPattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", CorePattern)
	}
	return nil
}

// EnableCrashFiltering enables crash filtering with the specified process.
func EnableCrashFiltering(s string) error {
	return replaceCrashFilterIn(s)
}

// DisableCrashFiltering removes the --filter_in argument from the kernel core dump cmdline.
// Next time the crash reporter is invoked (due to a crash) it will not receive a
// --filter_in paramter.
func DisableCrashFiltering() error {
	return replaceCrashFilterIn("")
}
