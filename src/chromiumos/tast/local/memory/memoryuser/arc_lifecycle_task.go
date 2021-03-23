// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// ArcLifecycleTask launches an Android app which allocates memory.
type ArcLifecycleTask struct {
	id            int
	allocateBytes int64
	ratio         float64
	limit         memory.Limit
}

// ArcLifecycleTask is a MemoryTest.
var _ MemoryTask = (*ArcLifecycleTask)(nil)

const arcLifecycleTestPkg = "org.chromium.arc.testapp.lifecycle"

func (t *ArcLifecycleTask) packageName() string {
	// E.g. "org.chromium.arc.testapp.lifecycle00".
	return fmt.Sprintf("%s%02d", arcLifecycleTestPkg, t.id)
}

func (t *ArcLifecycleTask) mainActivity() string {
	// E.g. "org.chromium.arc.testapp.lifecycle00/org.chromium.arc.testapp.lifecycle.MainActivity".
	return fmt.Sprintf("%s/%s.MainActivity", t.packageName(), arcLifecycleTestPkg)
}

func (t *ArcLifecycleTask) intentAction(action string) string {
	// E.g. "org.chromium.arc.testapp.lifecycle00.ALLOC".
	return fmt.Sprintf("%s.%s", t.packageName(), action)
}

var amStartRE = regexp.MustCompile("(?m)^Status: ok$")

// Run starts the AndroidLifecycleTest app, and uses it to allocate memory.
func (t *ArcLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	// Android doesn't like launching apps with the display asleep...
	if err := power.TurnOnDisplay(ctx); err != nil {
		return errors.Wrap(err, "failed to turn on display")
	}
	if t.limit != nil {
		// Limit has been provided, wait until we are not limited.
		const limitTimeout = 10 * time.Second
		if err := testing.Poll(ctx, t.limit.AssertNotReached, &testing.PollOptions{Interval: 1 * time.Second, Timeout: limitTimeout}); err != nil {
			return errors.Wrap(err, "failed to wait for memory to not be above limit")
		}
	}

	// Launch the app.
	testing.ContextLogf(ctx, "Starting %s", t.mainActivity())
	sizeString := strconv.FormatInt(t.allocateBytes, 10)
	ratioString := strconv.FormatFloat(t.ratio, 'f', -1, 64)
	startOut, err := testEnv.arc.Command(
		ctx,
		"am", "start", "-W", "-n", t.mainActivity(),
		"--el", "size", sizeString,
		"--ef", "ratio", ratioString,
	).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to start %q", t.mainActivity())
	}
	if !amStartRE.MatchString(string(startOut)) {
		return errors.Errorf("failed to start AndroidLifecycleTask, output %q", string(startOut))
	}

	// Wait for allocation to complete.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		done, err := t.doneAllocating(ctx, testEnv.arc)
		if err != nil {
			return err
		}
		if !done {
			return errors.New("allocation is not done")
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 30 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to allocate with %q", t.packageName())
	}
	return nil
}

// Close kills the AndroidLifecycleTest app.
func (t *ArcLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {
	err := testEnv.arc.Command(ctx, "am", "force-stop", t.packageName()).Run(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to Close %q: %v", t.packageName(), err)
	}
}

// String returns a friendly string containing the
func (t *ArcLifecycleTask) String() string {
	return fmt.Sprintf("ARC lifecycle%02d", t.id)
}

// NeedVM returns false, because we don't need Crostini.
func (t *ArcLifecycleTask) NeedVM() bool {
	return false
}

func (t *ArcLifecycleTask) doneAllocating(ctx context.Context, arc *arc.ARC) (bool, error) {
	data, err := arc.BroadcastIntentGetData(ctx, t.intentAction("DONE"))
	if err != nil {
		return false, err
	}
	done, err := strconv.ParseBool(string(data))
	if err != nil {
		return false, errors.Errorf("failed to parse DONE intent data %s: %s", string(data), err)
	}
	return done, nil
}

// StillAlive uses ps to see if the App is still running.
func (t *ArcLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	_, err := t.doneAllocating(ctx, testEnv.arc)
	return err == nil
}

// NewArcLifecycleTask creates a MemoryTask that runs the ArcLifecycleTest app
// and uses it to allocate memory.
//  id            - Which copy of ArcLifecycleTest app to use.
//  allocateBytes - mow much memory to allocate.
//  ratio         - the compression ratio of allocated memory.
//  limit         - if not nil, wait for Limit after allocation.
func NewArcLifecycleTask(id int, allocateBytes int64, ratio float64, limit memory.Limit) *ArcLifecycleTask {
	return &ArcLifecycleTask{id, allocateBytes, ratio, limit}
}

func androidLifecycleTestAPKPath(id int) string {
	return fmt.Sprintf("/usr/local/libexec/tast/apks/local/cros/ArcLifecycleTest%02d.apk", id)
}

// InstallArcLifecycleTestApps installs 'howMany' copies of ArcLifecycleTest.
func InstallArcLifecycleTestApps(ctx context.Context, a *arc.ARC, num int) error {
	testing.ContextLogf(ctx, "Installing %d copies of AndroidLifecycleTest app", num)
	for i := 0; i < num; i++ {
		if err := a.Install(ctx, androidLifecycleTestAPKPath(i)); err != nil {
			return errors.Wrapf(err, "failed to install %q", androidLifecycleTestAPKPath(i))
		}
	}
	testing.ContextLog(ctx, "Done")
	return nil
}
