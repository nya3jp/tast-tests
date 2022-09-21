// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package procutil

import (
	"sort"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
)

// Matcher is the interface to be passed to Find*() family,
// representing a condition to identify the target process.
type Matcher func(p *process.Process) bool

// ErrNotFound is an error returned by Find*() family, if no target process is
// found.
var ErrNotFound = errors.New("process not found")

// FoundTooManyProcessesError is returned by FindUnique, if there are two or more
// processes are found.
type FoundTooManyProcessesError struct {
	// E allows this error to be a part of Tast's errors library.
	*errors.E

	// Found holds all found processes.
	Found []*process.Process

	// All holds all processes on checking.
	All []*process.Process
}

// FindUnique returns a process.Process instance which is matched with the
// given matcher. If not found, or multiple processes satisfy the matcher,
// this returns an error. Specifically, on not found case, ErrNotFound is
// returned.
func FindUnique(m Matcher) (*process.Process, error) {
	found, all, err := findAllInternal(m)
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		pids := make([]int32, len(found))
		for i, proc := range found {
			pids[i] = proc.Pid
		}
		// Sort just for better human log readability.
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		return nil, &FoundTooManyProcessesError{
			E:     errors.Errorf("too many processes are found: %v", pids),
			Found: found,
			All:   all,
		}
	}
	return found[0], nil
}

// FindAll returns a list of all process.Process instances which are matched with
// the given matcher.
// If process it not found, this returns ErrNotFound.
func FindAll(m Matcher) ([]*process.Process, error) {
	found, _, err := findAllInternal(m)
	return found, err
}

func findAllInternal(m Matcher) (found, all []*process.Process, err error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, nil, err
	}

	var ret []*process.Process
	for _, p := range ps {
		if m(p) {
			ret = append(ret, p)
		}
	}

	if len(ret) == 0 {
		return nil, nil, ErrNotFound
	}
	return ret, ps, nil
}

// And is a utility to compose matchers into one matcher, which is satisfied
// only when all given matchers are satisfied.
// If nothing is passed, the returned matcher will match with any process.
func And(ms ...Matcher) Matcher {
	return func(p *process.Process) bool {
		for _, m := range ms {
			if !m(p) {
				return false
			}
		}
		return true
	}
}

// ByExe returns a Matcher which will be satisfied if a process's Exe
// is the same with the given path.
// exePath should be always absolute path.
func ByExe(exePath string) Matcher {
	return func(p *process.Process) bool {
		// Ignore any errors (i.e., handle them as a common not-matched
		// case), because the process may be terminated between the
		// instance creation and this Exe() invocation.
		exe, err := p.Exe()
		return err == nil && exe == exePath
	}
}
