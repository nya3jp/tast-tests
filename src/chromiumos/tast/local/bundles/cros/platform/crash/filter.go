// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"io/ioutil"

	ccrash "chromiumos/tast/common/crash"
	"chromiumos/tast/errors"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"
)

// replaceCrashFilterIn sets up the crash reporter to handle only some specific
// programs when a process dies. If param is an empty, all crashes are handled.
// This is used to ignore crashes in which we have no interest.
// See ReplaceCrashFilterString in chromiumos/tast/common/crash/filter.go for more details.
func replaceCrashFilterIn(param string) error {
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	newPattern, err := ccrash.ReplaceCrashFilterString(string(b), param)
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
