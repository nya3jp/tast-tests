// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cujrunner implements a way to run composed cuj using a json config.
package cujrunner

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type task struct {
	// Action to be performed for this task.
	a action
	// A task blocked by this task.
	blocked *task

	// Absolute delta to be added to runner start time to determine when this
	// task should run.
	st time.Duration
	// Relative delta to be added to the finish time of the block task to
	// determine when this task should run.
	rt time.Duration
}

type tasks []*task

func (ts tasks) Len() int           { return len(ts) }
func (ts tasks) Swap(i, j int)      { ts[i], ts[j] = ts[j], ts[i] }
func (ts tasks) Less(i, j int) bool { return ts[i].st < ts[j].st }

func createTask(a action) (*task, error) {
	t := &task{a: a}

	if a.Start != "" {
		// Non-empty start time. If started with '+', it is considered as a relative
		// time delta after the previous action finishes. Otherwise, it is
		// considered as an absolute time delta after the test starts.
		d, err := time.ParseDuration(a.Start)
		if err != nil {
			return nil, err
		}

		if a.Start[0] == '+' {
			t.rt = d
		} else {
			t.st = d
		}
	} else {
		// Empty start time is considered as a relative time delta 1ms after the
		// previous action finishes.
		t.rt, _ = time.ParseDuration("1ms")
	}

	return t, nil
}

// CUJRunner creates and runs tasks for actions defined in a JSON config.
type CUJRunner struct {
	cr *chrome.Chrome
	q  tasks
}

// NewRunner creates an instance of CUJRunner.
func NewRunner(cr *chrome.Chrome) *CUJRunner {
	r := &CUJRunner{cr: cr}
	return r
}

func (r *CUJRunner) getTask() *task {
	if len(r.q) == 0 {
		return nil
	}

	var t *task
	t, r.q = r.q[0], r.q[1:]
	return t
}

// Run runs the given json config.
func (r *CUJRunner) Run(ctx context.Context, s *testing.State, conf string) error {
	cb, err := ioutil.ReadFile(conf)
	if err != nil {
		return errors.Wrap(err, "failed to read conf file")
	}

	var actions []action
	if err := json.Unmarshal(cb, &actions); err != nil {
		return errors.Wrap(err, "failed to parse json conf file")
	}

	// Create tasks for actions defined in JSON and sort them by start time.
	var lt *task
	for _, a := range actions {
		t, err := createTask(a)
		if err != nil {
			return errors.Wrapf(err, "failed to create task for action: %s", a.Name)
		}

		if t.rt != 0 && lt != nil {
			lt.blocked = t
		} else {
			if t.rt != 0 {
				t.st = t.rt
			}

			r.q = append(r.q, t)
		}
		lt = t
	}
	sort.Sort(r.q)

	tconn, err := r.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// TODO(crbug/1113053): Needs a way to input what metrics to measure and
	// run inside cuj/Recorder.Run().

	// Run tasks by start time.
	st := time.Now()
	for t := r.getTask(); t != nil; t = r.getTask() {
		action, ok := getAction(t.a.Name)
		if !ok {
			return errors.Errorf("unknown action: %v", t.a.Name)
		}

		expectedStart := st.Add(t.st)
		sleepTime := expectedStart.Sub(time.Now())
		testing.ContextLogf(ctx, "Scheduling action %s, delay=%v", t.a.Name, sleepTime)
		if sleepTime > 0 {
			if err := testing.Sleep(ctx, sleepTime); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}

		cleanup, err := action(ctx, s, r.cr, tconn, t.a.Args)
		if err != nil {
			return errors.Wrapf(err, "failed to run action %s", t.a.Name)
		}
		if cleanup != nil {
			defer cleanup()
		}

		if t.blocked != nil {
			t.blocked.st = time.Now().Sub(st) + t.blocked.rt
			r.q = append(r.q, t.blocked)
			sort.Sort(r.q)
		}
	}

	return nil
}
