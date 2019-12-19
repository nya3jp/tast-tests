// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
TBD
*/

import (
	"context"
	"encoding/json"
	"sync"

	"chromiumos/tast/errors"
)

var runners map[string](StressTaskRunner)

func init() {
	runners = make(map[string](StressTaskRunner))
}

// RegisterRunner registers |r| with |name| for the lookup afterwards.
func RegisterRunner(r StressTaskRunner, name string) {
	runners[name] = r
}

// StressTaskRunner is the interface that has RunTask() method.
type StressTaskRunner interface {
	RunTask(ctx context.Context) error
}

// StressTestStatus holds the status of a test piece, static or dynamic
type StressTestStatus struct {
	//Name is the name of the test piece
	Name string
	// Count is the counter to record the current test iteration number.
	Count int
	// Errors records the error occurring during stress test (#iteration -> error)
	Errors map[int]error
}

// NewStressTestStatus creates a new status with |name|
func NewStressTestStatus(name string) *StressTestStatus {
	return &StressTestStatus{name, 0, make(map[int]error)}
}

// StressTestCriteria is the interface that defines the running criteria of a stress test.
type StressTestCriteria interface {
	ShallRun() bool
}

// StressTestKRun is a StressTestCriteria where the criteria is a fixed number of runs.
type StressTestKRun struct {
	status *StressTestStatus
	bound  int
}

// ShallRun implements the one of StressTestCriteria.
func (s *StressTestKRun) ShallRun() bool {
	return s.status.Count < s.bound
}

// StressTestStopOnFail is a StressTestCriteria where the test stops upon any error occurs in the previous iterations.
type StressTestStopOnFail struct {
	status *StressTestStatus
}

// ShallRun implements the one of StressTestCriteria.
func (s *StressTestStopOnFail) ShallRun() bool {
	return len(s.status.Errors) == 0
}

// StressTestAndPolicy is a StressTestCriteria of a intersection of all sub-criteria.
type StressTestAndPolicy struct {
	c []StressTestCriteria
}

// ShallRun implements the one of StressTestCriteria.
func (s *StressTestTrue) ShallRun() bool {
	return true
}

// StressTestTrue is a StressTestCriteria that indicates infinite runs.
type StressTestTrue struct {
}

// ShallRun implements the one of StressTestCriteria.
func (s *StressTestAndPolicy) ShallRun() bool {
	for _, c := range s.c {
		if !c.ShallRun() {
			return false
		}
	}
	return true
}

// StressTestOrPolicy is a StressTestCriteria of a union of all sub-criteria.
type StressTestOrPolicy struct {
	c []StressTestCriteria
}

// ShallRun implements the one of StressTestCriteria.
func (s *StressTestOrPolicy) ShallRun() bool {
	for _, c := range s.c {
		if !c.ShallRun() {
			return true
		}
	}
	return false
}

// StressTaskSequence is a |StressTaskRunner| that runs a sequence of |StressTaskRunner|s.
type StressTaskSequence struct {
	tasks [](StressTaskRunner)
}

// RunTask implements the one of |StressTaskRunner|.
func (s *StressTaskSequence) RunTask(ctx context.Context) error {
	for _, subTask := range s.tasks {
		if err := subTask.RunTask(ctx); err != nil {
			panic(err)
			return err
		}
	}
	return nil
}

// NewStressTaskSequence creates a new StressTaskSequence of |tasks|.
func NewStressTaskSequence(tasks [](StressTaskRunner)) *StressTaskSequence {
	return &StressTaskSequence{tasks}
}

// StressTester runs  a stress test associated with runner.
type StressTester struct {
	runner StressTaskRunner
	st     *StressTestStatus
	c      StressTestCriteria
}

// Run runs the stress test; upon finishing, potionally notify |wg| if it exists.
func (st *StressTester) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	for st.c.ShallRun() {
		if err := st.runner.RunTask(ctx); err != nil {
			st.st.Errors[st.st.Count] = err
		}
		st.st.Count = st.st.Count + 1
	}
}

// StressTaskRunnerConcurrentOnce is StressTaskRunner that runs all the stress tests concurrently.
type StressTaskRunnerConcurrentOnce struct {
	testers []*StressTester
}

// RunTask implements the one of StressTaskRunner.
func (s *StressTaskRunnerConcurrentOnce) RunTask(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(len(s.testers))
	for _, st := range s.testers {
		go st.Run(ctx, &wg)

	}
	return nil
}

// NewConcurrentStressTester creates a new StressTester that runs all the |sts| concurrently.
func NewConcurrentStressTester(sts ...*StressTester) *StressTester {
	topRunner := &StressTaskRunnerConcurrentOnce{sts}
	topStatus := NewStressTestStatus("Top")
	topCriteria := &StressTestKRun{topStatus, 1}
	return &StressTester{topRunner, topStatus, topCriteria}
}

// NewSimpleStressTester creates a new counter-based StressTester; optionally the test stops on fail depending on |stopOnFail| flag.
func NewSimpleStressTester(r StressTaskRunner, name string, count int, stopOnFail bool) *StressTester {
	st := NewStressTestStatus(name)
	var c StressTestCriteria = &StressTestKRun{st, count}
	if stopOnFail {
		c = &StressTestAndPolicy{[]StressTestCriteria{c, &StressTestStopOnFail{st}}}
	}
	return &StressTester{r, st, c}
}

// TaskModelDesc describes the static information of a task.
type TaskModelDesc struct {
	Names      []string `json:"names"`
	Count      *int     `json:"count"`
	StopOnFail bool     `json:"cof"`
}

// PSTaskModelDesc describes the static information of a primary-secondary task model.
type PSTaskModelDesc struct {
	Primary   TaskModelDesc   `json:"primary"`
	Secondary []TaskModelDesc `json:"secondary`
}

// UnmarshalTaskModel unmarshal |j| JSON string into a simple stress tester.
func UnmarshalTaskModel(j string) (*StressTester, error) {
	var tm TaskModelDesc
	if err := json.Unmarshal([]byte(j), &tm); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json into task model")
	}
	return fromTaskModelDesc(tm)
}

// UnmarshalPSTaskModel unmarshal |j| JSON string into a concurrent stress tester in a primary-secondary task model.
func UnmarshalPSTaskModel(j string) (*StressTester, error) {
	var tm PSTaskModelDesc
	if err := json.Unmarshal([]byte(j), &tm); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json into P-S task model")
	}
	return fromPSTaskModelDesc(tm)
}

func fromTaskModelDesc(tm TaskModelDesc) (*StressTester, error) {
	if len(tm.Names) == 0 {
		return nil, errors.New("no task specified")
	}
	// Constructs the runner sequence and the name
	n := ""
	var rSeq []StressTaskRunner
	for _, name := range tm.Names {
		if n != "" {
			n += "-"
		}
		n += name
		r, ok := runners[name]
		if !ok {
			return nil, errors.New("No entry: " + name)
		}
		rSeq = append(rSeq, r)
	}
	var r StressTaskRunner
	if len(rSeq) == 1 {
		r = rSeq[0]
	} else {
		r = NewStressTaskSequence(rSeq)
	}
	// Construct the status
	st := NewStressTestStatus(n)
	// Now the criteria
	var c1, c2 StressTestCriteria
	if tm.Count != nil {
		c1 = &StressTestKRun{st, *tm.Count}
	}
	if tm.StopOnFail {
		c2 = &StressTestStopOnFail{st}
	}
	c := c1
	switch {
	case c1 == nil && c2 == nil:
		c = &StressTestTrue{}
	case c1 == nil && c2 != nil:
		c = c2
	case c2 != nil:
		c = &StressTestAndPolicy{[]StressTestCriteria{c1, c2}}
	}

	// Last, construct the stress tester
	return &StressTester{r, st, c}, nil
}

func fromPSTaskModelDesc(tm PSTaskModelDesc) (*StressTester, error) {
	// Sanity-checks the counter conditions.
	if tm.Primary.Count == nil {
		return nil, errors.New("infinite runs of primary tasks is forbidden")
	}
	for _, m := range tm.Secondary {
		if m.Count != nil {
			return nil, errors.New("secondary tasks bounded by a infinite number of runs")
		}
	}
	p, err := fromTaskModelDesc(tm.Primary)
	if err != nil {
		return nil, errors.Wrap(err, "error creating primary task")
	}
	var sList [](*StressTester)
	for _, m := range tm.Secondary {
		s, err := fromTaskModelDesc(m)
		if err != nil {
			return nil, errors.Wrap(err, "error creating secondary task")
		}
		sList = append(sList, s)
	}
	// Wires the criteria of secondary tasks to primary task's via AND policy.
	// This way, secondary tasks stop once primary task does.
	for _, t := range sList {
		t.c = &StressTestAndPolicy{[]StressTestCriteria{t.c, p.c}}
	}
	return NewConcurrentStressTester(append(sList, p)...), nil
}
