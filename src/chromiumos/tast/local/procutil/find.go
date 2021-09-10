// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package procutil

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// Matcher is the interface to be passed to Find*() family,
// representing a condition to identify the target process.
type Matcher func(p *process.Process) bool

// FindUnique returns a process.Process instance which is matched with all
// matchers. (i.e. conditions are combined in "AND" way).
// If not found, or multiple processes satisfy the matchers, this returns
// an error.
func FindUnique(m Matcher, ms ...Matcher) (*process.Process, error) {
	ps, err := FindAll(m, ms...)
	if err != nil {
		return nil, err
	}
	if len(ps) != 1 {
		pids := make([]int32, len(ps))
		for i, proc := range ps {
			pids[i] = proc.Pid
		}
		return nil, errors.Errorf("too many processes are found; %v", pids)
	}
	return ps[0], nil
}

// FindAll returns a list of all process.Process instances which are matched with all
// |ms| matchers.
// At least one Matcher needs to be passed. To obtain all running processes,
// process.Processes() should just work.
func FindAll(m Matcher, ms ...Matcher) ([]*process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var ret []*process.Process
	matchers := append([]Matcher{m}, ms...)
	for _, p := range ps {
		if matches(p, matchers) {
			ret = append(ret, p)
		}
	}

	if len(ret) == 0 {
		return nil, errors.New("process not found")
	}
	return ret, nil
}

// matches returns whether p satisfies all ms matchers.
func matches(p *process.Process, ms []Matcher) bool {
	for _, m := range ms {
		if !m(p) {
			return false
		}
	}
	return true
}

// ByExe returns a Matcher which will be satisfied if a process's Exe
// is the same with the given path.
// exePath should be absolute path.
func ByExe(exePath string) Matcher {
	return func(p *process.Process) bool {
		// Ignore any errors (i.e., handle them as a common not-matched
		// case), because the process may be terminated between the
		// instance creation and this Exe() invocation.
		exe, err := p.Exe()
		return err == nil && exe == exePath
	}
}
