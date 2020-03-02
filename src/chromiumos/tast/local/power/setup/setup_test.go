// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"testing"

	"chromiumos/tast/errors"
)

type testSetupCleanup struct {
	sIndex int
	cIndex int
}

func newTestSetupCleanup() *testSetupCleanup {
	return &testSetupCleanup{
		sIndex: -1,
		cIndex: -1,
	}
}

// nextIndex gives a unique number to setup and cleanup events.
var nextIndex = 0

func (tsc *testSetupCleanup) setup() (CleanupCallback, error) {
	tsc.sIndex = nextIndex
	nextIndex++
	return func(_ context.Context) error {
		tsc.cIndex = nextIndex
		nextIndex++
		return nil
	}, nil
}

func (tsc *testSetupCleanup) check(shouldHaveRun bool) error {
	if shouldHaveRun {
		if tsc.sIndex == -1 {
			return errors.New("setup did not run")
		}
		if tsc.cIndex == -1 {
			return errors.New("cleanup did not run")
		}
	} else {
		if tsc.sIndex != -1 {
			return errors.New("setup ran when it should not have")
		}
		if tsc.cIndex != -1 {
			return errors.New("cleanup ran when it should not have")
		}
	}
	return nil
}

func (tsc *testSetupCleanup) cleanedBefore(other *testSetupCleanup) bool {
	return tsc.cIndex < other.cIndex
}

func emptySetup() (CleanupCallback, error) {
	return nil, nil
}

var errSetup = errors.New("error in setup")
var errCleanup = errors.New("error in cleanup")

func errorInSetup() (CleanupCallback, error) {
	return nil, errSetup
}

func errorInCleanup() (CleanupCallback, error) {
	return func(_ context.Context) error {
		return errCleanup
	}, nil
}

func panicInSetup() (CleanupCallback, error) {
	panic(errSetup)
}

func panicInCleanup() (CleanupCallback, error) {
	return func(_ context.Context) error {
		panic(errCleanup)
	}, nil
}

func TestSetup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, tc := range []struct {
		testItem                            func() (CleanupCallback, error)
		expectedPanic                       error
		setupFails, cleanupFails, item1Runs bool
	}{
		// Normal setup without errors.
		{emptySetup, nil, false, false, true},
		// Error in middle setup, item0 and item1 both setup and cleanup.
		{errorInSetup, nil, true, false, true},
		// Error in middle cleanup, item0 and item1 both setup and cleanup.
		{errorInCleanup, nil, false, true, true},
		// Panic in middle setup, only item0 has setup and cleanup.
		{panicInSetup, errSetup, false, false, false},
		// Panic in middle cleanup, item0 and item1 both setup and cleanup.
		{panicInCleanup, errCleanup, false, false, true},
	} {
		item0 := newTestSetupCleanup()
		item1 := newTestSetupCleanup()

		func() {
			s, c := New()
			defer func() {
				if v := recover(); v != tc.expectedPanic {
					panic(v)
				}
			}()
			defer func() {
				if err := c(ctx); err != nil && !tc.cleanupFails {
					t.Error("Failure in cleanup: ", err)
				}
			}()
			s.Add(item0.setup())
			s.Add(tc.testItem())
			s.Add(item1.setup())
			if err := s.Check(ctx); err != nil && !tc.setupFails {
				t.Error("Failure in setup: ", err)
			}
		}()

		if err := item0.check(true); err != nil {
			t.Error("Item 0 failed: ", err)
		}
		if err := item1.check(tc.item1Runs); err != nil {
			t.Error("Item 1 failed: ", err)
		}
		if tc.item1Runs && item0.cleanedBefore(item1) {
			t.Error("Item 0 wasn't cleaned after item 1")
		}
	}
}

func TestNestedSetup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, tc := range []struct {
		testItem                   func() (CleanupCallback, error)
		expectedPanic, nestedError error
		setupFails, cleanupFails   bool
		cleanupOrder               []int
	}{
		// Normal setup, everything cleaned up in order.
		{emptySetup, nil, nil, false, false, []int{3, 2, 1, 0}},
		// Nested returns error, second nested item doesn't run, first cleans up
		// immediately.
		{emptySetup, nil, errSetup, true, false, []int{1, 3, 0}},
		// Error in middle nested setup, nested items clean up immediately,
		// outer items clean up at end.
		{errorInSetup, nil, nil, true, false, []int{2, 1, 3, 0}},
		// Error in middle nested cleanup, everything cleaned up in order.
		{errorInCleanup, nil, nil, false, true, []int{3, 2, 1, 0}},
		// Panic in middle nested setup, first nested cleaned up immediately,
		// first outer cleaned up at end.
		{panicInSetup, errSetup, nil, false, false, []int{1, 0}},
		// Panic in middle nested cleanup, everything cleaned up in order.
		{panicInCleanup, errCleanup, nil, false, false, []int{3, 2, 1, 0}},
	} {
		items := []*testSetupCleanup{
			newTestSetupCleanup(),
			newTestSetupCleanup(),
			newTestSetupCleanup(),
			newTestSetupCleanup(),
		}

		func() {
			s, c := New()
			defer func() {
				if v := recover(); v != tc.expectedPanic {
					panic(v)
				}
			}()
			defer func() {
				if err := c(ctx); err != nil && !tc.cleanupFails {
					t.Error("Failure in cleanup: ", err)
				}
			}()
			s.Add(items[0].setup())
			s.Add(Nested(ctx, "test", func(s *Setup) error {
				s.Add(items[1].setup())
				s.Add(tc.testItem())
				if tc.nestedError != nil {
					return tc.nestedError
				}
				s.Add(items[2].setup())
				return nil
			}))
			s.Add(items[3].setup())
		}()

		// Check that all setup and cleanup operations that should have run did.
		for i, item := range items {
			shouldHaveRun := false
			for _, j := range tc.cleanupOrder {
				if i == j {
					shouldHaveRun = true
				}
			}
			if err := item.check(shouldHaveRun); err != nil {
				t.Errorf("Item %d failed: %v", i, err)
			}
		}
		// Check the order of all cleanup operations.
		for ci := 0; ci < len(tc.cleanupOrder)-1; ci++ {
			i := tc.cleanupOrder[ci]
			j := tc.cleanupOrder[ci+1]
			if !items[i].cleanedBefore(items[j]) {
				t.Errorf("Item %d was cleaned before item %d", i, j)
			}
		}
	}
}

func TestMultipleCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, c := New()
	s.Add(emptySetup())
	if err := s.Check(ctx); err != nil {
		t.Error("Setup failed")
	}
	if err := c(ctx); err != nil {
		t.Error("First cleanup failed.")
	}
	if err := c(ctx); err == nil {
		t.Error("Second cleanup didn't fail.")
	}
}
