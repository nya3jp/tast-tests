// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "gpuWatchDog",
		Desc:            "Check if there any gpu related problems observed during a test.",
		Impl:            &gpuWatchDogFixture{},
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeGraphics",
		Desc:            "Logged into a user session for graphics testing.",
		Impl:            chrome.NewLoggedInFixture(),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

type gpuWatchDogFixture struct {
	hangPatterns []string
	postFunc     []func(ctx context.Context) error
}

func (f *gpuWatchDogFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// TODO: This needs to be kept in sync for new drivers, especially ARM.
	f.hangPatterns = []string{
		"drm:i915_hangcheck_elapsed",
		"drm:i915_hangcheck_hung",
		"Hangcheck timer elapsed...",
		"drm/i915: Resetting chip after gpu hang",
	}
	return nil
}

func (f *gpuWatchDogFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *gpuWatchDogFixture) Reset(ctx context.Context) error {
	return nil
}

// getGPUCrash returns gpu related crash files found in system.
func (f *gpuWatchDogFixture) getGPUCrash() ([]string, error) {
	crashFiles, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		return nil, err
	}
	// Filter the gpu related crash.
	var crashes []string
	for _, file := range crashFiles {
		if strings.HasSuffix(file, crash.GPUStateExt) {
			crashes = append(crashes, file)
		}
	}
	return crashes, nil
}

// checkNewCrashes checks the difference between the oldCrashes and the current crashes. Return error if failed to retrieve current crashes or the list is mismatch.
func (f *gpuWatchDogFixture) checkNewCrashes(ctx context.Context, oldCrashes []string) error {
	crashes, err := f.getGPUCrash()
	if err != nil {
		return err
	}

	// Check if there're new crash files got generated during the test.
	for _, crash := range crashes {
		found := false
		for _, preTestCrash := range oldCrashes {
			if preTestCrash == crash {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("found gpu crash file: %s", crash)
		}
	}
	return nil
}

// getGPUHang returns the gpu hang lines in messages file as a map.
func (f *gpuWatchDogFixture) getGPUHang() (map[string]bool, error) {
	out, err := ioutil.ReadFile(syslog.MessageFile)
	if err != nil {
		return nil, err
	}
	hangLine := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		for _, pattern := range f.hangPatterns {
			if strings.Contains(line, pattern) {
				hangLine[line] = true
			}
		}
	}
	return hangLine, nil
}

// checkNewHangs checks the oldHangs with the current hangs in syslog.MessageFile. It returns error if failed to read the file or the lines are mismatch.
func (f *gpuWatchDogFixture) checkNewHangs(ctx context.Context, oldHangs map[string]bool) error {
	hangs, err := f.getGPUHang()
	if err != nil {
		return err
	}

	for newhangs := range hangs {
		if _, ok := oldHangs[newhangs]; !ok {
			return errors.New("detected a gpu hang during test")
		}
	}
	return nil
}

func (f *gpuWatchDogFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.postFunc = nil
	// Attempt flushing system logs every second instead of every 10 minutes.
	dirtyWritebackDuration, err := GetDirtyWritebackDuration()
	if err != nil {
		s.Log("Failed to set get dirty writeback duration: ", err)
	} else {
		if err := SetDirtyWritebackDuration(ctx, 1*time.Second); err != nil {
			f.postFunc = append(f.postFunc, func(ctx context.Context) error {
				s.Log("set back dirty writeback")
				return SetDirtyWritebackDuration(ctx, dirtyWritebackDuration)
			})
		}
	}

	// Record PreTest crashes.
	crashes, err := f.getGPUCrash()
	if err != nil {
		s.Log("Failed to get gpu crashes: ", err)
	} else {
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return f.checkNewCrashes(ctx, crashes)
		})
	}

	// Record PreTest GPU hangs.
	hangLine, err := f.getGPUHang()
	if err != nil {
		s.Log("Failed to get GPU hang line: ", err)
	} else {
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return f.checkNewHangs(ctx, hangLine)
		})
	}
}

func (f *gpuWatchDogFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	var postErr error
	for i := len(f.postFunc) - 1; i >= 0; i-- {
		if err := f.postFunc[i](ctx); err != nil {
			postErr = errors.Wrap(postErr, err.Error())
		}
	}
	if postErr != nil {
		s.Error("PostTest failed: ", postErr)
	}
}
