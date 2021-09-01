// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "gpuWatchHangs",
		Desc:            "Check if there any gpu related hangs during a test",
		Contacts:        []string{"ddmail@google.com", "chromeos-gfx@google.com"},
		Impl:            &gpuWatchHangsFixture{},
		PreTestTimeout:  30 * time.Second,
		PostTestTimeout: 30 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "gpuWatchDog",
		Desc:            "Check if there any gpu related problems(hangs+crashes) observed during a test",
		Contacts:        []string{"ddmail@google.com", "chromeos-gfx@google.com"},
		Parent:          "gpuWatchHangs",
		Impl:            &gpuWatchDogFixture{},
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeGraphics",
		Desc:     "Logged into a user session for graphics testing",
		Contacts: []string{"ddmail@google.com", "chromeos-gfx@google.com"},
		Parent:   "gpuWatchDog",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeGraphicsLacros",
		Desc:     "Logged into a user session for graphics testing (lacros)",
		Contacts: []string{"lacros-team@google.com"},
		Parent:   "gpuWatchDog",
		Impl: launcher.NewStartedByData(launcher.PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{launcher.DataArtifact},
		Vars:            []string{launcher.LacrosDeployedBinary},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "graphicsNoChrome",
		Desc:            "Stop UI before tests, start UI after",
		Contacts:        []string{"chromeos-gfx@google.com"},
		Impl:            &graphicsNoChromeFixture{},
		Parent:          "gpuWatchHangs",
		SetUpTimeout:    upstart.UIRestartTimeout,
		TearDownTimeout: upstart.UIRestartTimeout,
	})
}

type graphicsNoChromeFixture struct {
}

func (f *graphicsNoChromeFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *graphicsNoChromeFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *graphicsNoChromeFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *graphicsNoChromeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("Setup: Stop Chrome UI")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}
	return nil
}

func (f *graphicsNoChromeFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("Setup: Start Chrome UI")
	upstart.EnsureJobRunning(ctx, "ui")
}

type gpuWatchHangsFixture struct {
	regexp   *regexp.Regexp
	postFunc []func(ctx context.Context) error
}

func (f *gpuWatchHangsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// TODO: This needs to be kept in sync for new drivers, especially ARM.
	hangRegexStrs := []string{
		`drm:i915_hangcheck_elapsed`,
		`drm:i915_hangcheck_hung`,
		`Hangcheck timer elapsed...`,
		`drm/i915: Resetting chip after gpu hang`,
		`GPU HANG:.+\b[H|h]ang on (rcs0|vcs0|vecs0)`,
		`hangcheck recover!`,      // Freedreno
		`mtk-mdp.*: cmdq timeout`, // Mediatek
		`amdgpu: GPU reset begin!`,
	}
	// TODO(pwang): add regex for memory faults.
	f.regexp = regexp.MustCompile(strings.Join(hangRegexStrs, "|"))
	s.Log("Setup regex to detect GPU hang: ", f.regexp)
	return nil
}

func (f *gpuWatchHangsFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *gpuWatchHangsFixture) Reset(ctx context.Context) error {
	return nil
}

// checkHangs checks gpu hangs from the reader. It returns error if failed to read the file or gpu hang patterns are detected.
func (f *gpuWatchHangsFixture) checkHangs(ctx context.Context, reader *syslog.Reader) error {
	for {
		e, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "failed to read syslog")
		}

		matches := f.regexp.FindAllStringSubmatch(e.Line, -1)
		if len(matches) > 0 {
			return errors.Errorf("GPU hang: %s", e.Line)
		}
	}
	return nil
}

func (f *gpuWatchHangsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.postFunc = nil
	// Attempt flushing system logs every second instead of every 10 minutes.
	dirtyWritebackDuration, err := GetDirtyWritebackDuration()
	if err != nil {
		s.Log("Failed to get initial dirty writeback duration: ", err)
	} else {
		SetDirtyWritebackDuration(ctx, 1*time.Second)
		// Set dirty writeback duration to initial value even if we fails to set to 1 second. Note this implicitly calls sync.
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return SetDirtyWritebackDuration(ctx, dirtyWritebackDuration)
		})
	}
	// syslog.NewReader reports syslog message written after it is started for GPU hang detection.
	sysLogReader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Log("Failed to get syslog reader: ", err)
	} else {
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			defer sysLogReader.Close()
			return f.checkHangs(ctx, sysLogReader)
		})
	}
}

func (f *gpuWatchHangsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
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

type gpuWatchDogFixture struct {
	postFunc []func(ctx context.Context) error
}

func (f *gpuWatchDogFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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

// checkNewCrashes checks the difference between the oldCrashes and the current crashes. It will try to save the new crash to outDir and return error if fails to retrieve current crashes or the list is mismatched.
func (f *gpuWatchDogFixture) checkNewCrashes(ctx context.Context, oldCrashes []string, outDir string) error {
	crashes, err := f.getGPUCrash()
	if err != nil {
		return err
	}

	// Check if there're new crash files got generated during the test.
	var newCrashes []string
	for _, crash := range crashes {
		found := false
		for _, preTestCrash := range oldCrashes {
			if preTestCrash == crash {
				found = true
				break
			}
		}
		if !found {
			newCrashes = append(newCrashes, crash)
		}
	}

	if len(newCrashes) > 0 {
		sort.Strings(newCrashes)
		resultErr := errors.Errorf("found gpu crash file: %v", newCrashes)
		for _, crash := range newCrashes {
			destPath := filepath.Join(outDir, filepath.Base(crash))
			if err := fsutil.CopyFile(crash, destPath); err != nil {
				resultErr = errors.Wrapf(resultErr, "failed to copy crash file %v: %v", crash, err.Error())
			}
		}
		return resultErr
	}
	return nil
}

func (f *gpuWatchDogFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.postFunc = nil
	// Record PreTest crashes.
	crashes, err := f.getGPUCrash()
	if err != nil {
		s.Log("Failed to get gpu crashes: ", err)
	} else {
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return f.checkNewCrashes(ctx, crashes, s.OutDir())
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
