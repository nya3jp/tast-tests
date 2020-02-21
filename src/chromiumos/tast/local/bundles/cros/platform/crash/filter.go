// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"io/ioutil"
	"strconv"
	"strings"

	"chromiumos/tast/common/crash"
	"chromiumos/tast/errors"
)

// replaceArgs finds commandline arguments that match by prefix and replaces it with newarg.
// TODO(yamaguchi): Deduplicate with the one in chromiumos/tast/local/crash/filter.go
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

// ReporterVerboseLevel sets the output log verbose level of the crash_reporter.
// When level is set to 0, it will clear the -v=* flag because 0 is the default value.
func ReporterVerboseLevel(level int) error {
	if level < 0 {
		return errors.Errorf("verbose level must be 0 or larger, got %d", level)
	}
	b, err := ioutil.ReadFile(crash.CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", crash.CorePattern)
	}
	pattern := string(b)
	newarg := ""
	if level > 0 {
		newarg = "-v=" + strconv.Itoa(level)
	}
	pattern = replaceArgs(pattern, "-v=", newarg)
	if err := ioutil.WriteFile(crash.CorePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", crash.CorePattern)
	}
	return nil
}
