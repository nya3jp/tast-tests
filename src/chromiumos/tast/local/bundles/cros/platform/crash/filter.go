// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"io/ioutil"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"
)

// replaceArgs finds commandline arguments that match by prefix and replaces it with newarg.
func replaceArgs(orig string, prefix string, newarg string) string {
	e := strings.Fields(strings.TrimSpace(orig))
	var newargs []string
	replaced := false
	for _, s := range e {
		if !strings.HasPrefix(s, prefix) {
			newargs = append(newargs, s)
			continue
		}
		if len(newarg) == 0 {
			// Remove from list.
			continue
		}
		newargs = append(newargs, newarg)
		replaced = true
	}
	if len(newarg) != 0 && !replaced {
		newargs = append(newargs, newarg)
	}
	return strings.Join(newargs, " ")
}

// replaceCrashFilterIn replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// The kernel is set up to call the crash reporter with the core dump as stdin
// when a process dies. This function adds a filter to the command line used to
// call the crash reporter. This is used to ignore crashes in which we have no
// interest.
func replaceCrashFilterIn(param string) error {
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	pattern := string(b)
	if !strings.HasPrefix(pattern, "|") {
		return errors.Wrapf(err, "pattern should start with '|', but was: %s", pattern)
	}
	if len(param) == 0 {
		pattern = replaceArgs(pattern, "--filter_in=", "")
	} else {
		pattern = replaceArgs(pattern, "--filter_in=", "--filter_in="+param)
	}
	if err := ioutil.WriteFile(CorePattern, []byte(pattern), 0644); err != nil {
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

// ReporterVerboseLevel sets the output log verbose level of the crash_reporter.
// When level is set to 0, it will clear the -v=* flag because 0 is the default value.
func ReporterVerboseLevel(level int) error {
	if level < 0 {
		return errors.Errorf("verbose level must be 0 or larger, got %d", level)
	}
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	pattern := string(b)
	newarg := ""
	if level > 0 {
		newarg = "-v=" + strconv.Itoa(level)
	}
	pattern = replaceArgs(pattern, "-v=", newarg)
	if err := ioutil.WriteFile(CorePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", CorePattern)
	}
	return nil
}
