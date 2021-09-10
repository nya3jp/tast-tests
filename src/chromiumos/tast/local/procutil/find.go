// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package procutil

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// Matcher is the interface to be passed to Find*() family.
type Matcher func(p *process.Process) bool

// ErrNotFound is an error returned from Find*() family if the target process
// is not found.
var ErrNotFound = errors.New("process not found")

// Find returns a process.Process instance which is matched with all
// conditions. (i.e. conditions are combined in "AND" way).
// If not found, ErrNotFound is returned.
// If there are multiple processes satisfy the conditions, then arbitrary one
// will be returned. Also, in that case, there's no guarantee that the same
// process will be returned when Find() is called multiple times in a sequence
// with the same conditions, even if actual running processes are not changed
// on the system.
// This is convenient if it is known that at most one process that satisfies
// the conditions runs on the system. Otherwise, you may want to use FindAll()
// instead.
// At least one Matcher needs to be passed, otherwise just a random process
// would be returned.
func Find(m Matcher, ms ...Matcher) (*process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain processes")
	}

	matchers := append([]Matcher{m}, ms...)
	for _, p := range ps {
		if matches(p, matchers) {
			return p, nil
		}
	}

	return nil, ErrNotFound
}

// FindAll returns a list of process.Process instances which are mached with all
// |ms| conditions. This is similar to Find(), but instead of returning arbitrary
// one, this returns all instances which satisfy the all conditions.
// At leat one Matcher needs to be passed. To obtain all running processes,
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
		return nil, ErrNotFound
	}
	return ret, nil
}

// matches returns wehther p satisfies all ms conditions.
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
