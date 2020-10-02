// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ArcLifecycleTask launches an Android app which allocates memory.
type ArcLifecycleTask struct {
	id            int
	allocateBytes int64
	ratio         float32
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
	// E.g. "org.chromium.arc.testapp.lifecycle00/org.chromium.arc.testapp.lifecycle.ALLOC".
	return fmt.Sprintf("%s%02d/%s.MainActivity", arcLifecycleTestPkg, t.id, arcLifecycleTestPkg)
}

func (t *ArcLifecycleTask) intentAction(action string) string {
	// E.g. "org.chromium.arc.testapp.lifecycle00.ALLOC".
	return fmt.Sprintf("%s%02d.%s", arcLifecycleTestPkg, t.id, action)
}

var errAllocationNotDone = errors.New("allocation is not done")

// Run starts the AndroidLifecycleTest app, and uses it to allocate memory.
func (t *ArcLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	// Launch the app.
	if err := testEnv.arc.Command(ctx, "am", "start", "-W", t.mainActivity()).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to start %q", t.mainActivity())
	}

	// Allocate.
	sizeString := strconv.FormatInt(t.allocateBytes, 10)
	if _, err := testEnv.arc.BroadcastIntentGetData(ctx, t.intentAction("ALLOC"), "--el", "size", sizeString); err != nil {
		return errors.Wrapf(err, "failed to allocate with %q", t.packageName())
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		data, err := testEnv.arc.BroadcastIntentGetData(ctx, t.intentAction("DONE"))
		if err != nil {
			return errors.Wrapf(err, "failed to wait for %q to finish allocating", t.packageName())
		}
		if data != "0" {
			return errAllocationNotDone
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to allocate with %q", t.packageName())
	}
	if t.limit == nil {
		return nil
	}
	// Limit has been provided, wait until we are not limited.
	if err := testing.Poll(ctx, t.limit.Assert, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for memory to not be above limit")
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
	return fmt.Sprintf("%s, %d bytes", t.packageName(), t.allocateBytes)
}

// NeedVM returns false, because we don't need Crostini.
func (t *ArcLifecycleTask) NeedVM() bool {
	return false
}

// StillAlive checks to see if the app is still responding to broadcast intents.
func (t *ArcLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	_, err := testEnv.arc.BroadcastIntentGetData(ctx, t.intentAction("DONE"))
	return err == nil
}

// NewArcLifecycleTask creates a MemoryTask that runs the ArcLifecycleTest app
// and uses it to allocate memory.
//  id            - Which copy of ArcLifecycleTest app to use.
//  allocateBytes - mow much memory to allocate.
//  ratio         - the compression ratio of allocated memory.
//  limit         - if not nil, wait for Limit after allocation.
func NewArcLifecycleTask(id int, allocateBytes int64, ratio float32, limit memory.Limit) *ArcLifecycleTask {
	return &ArcLifecycleTask{id, allocateBytes, ratio, limit}
}

// BestEffortArcLifecycleTask is a ArcLifecycleTask that doesn't fail the test
// if the task fails, but instead allows metrics to be computed for successful
// tasks.
type BestEffortArcLifecycleTask struct {
	ArcLifecycleTask
	succeeded bool
}

// BestEffortArcLifecycleTask conforms to MemoryTask interface.
var _ MemoryTask = (*BestEffortArcLifecycleTask)(nil)

// BestEffortArcLifecycleTask conforms to KillableTask interface.
var _ KillableTask = (*BestEffortArcLifecycleTask)(nil)

// BestEffortArcLifecycleTask conforms to SilentFailTask interface.
var _ SilentFailTask = (*BestEffortArcLifecycleTask)(nil)

// Run runs the ArcLifecycleTask and logs any errors if they happen. Always
// returns nil error.
func (t *BestEffortArcLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	err := t.ArcLifecycleTask.Run(ctx, testEnv)
	t.succeeded = err == nil
	if err != nil {
		testing.ContextLogf(ctx, "Failed to Run %s: %v", t.String(), err)
	}
	return nil
}

// Succeeded is true if the most recent call to Run did not return an error.
func (t *BestEffortArcLifecycleTask) Succeeded() bool {
	return t.succeeded
}

// NewBestEffortArcLifecycleTask creates a BestEffort MemoryTask with the same
// parameters as NewArcLifecycleTask above.
func NewBestEffortArcLifecycleTask(id int, allocateBytes int64, ratio float32, limit memory.Limit) *BestEffortArcLifecycleTask {
	return &BestEffortArcLifecycleTask{
		ArcLifecycleTask{id, allocateBytes, ratio, limit},
		false,
	}
}

func androidLifecycleTestAPKPath(id int) string {
	return fmt.Sprintf("/usr/libexec/tast/apks/local/cros/ArcLifecycleTest%02d.apk", id)
}

// InstallArcLifecycleTestApps installs 'howMany' copies of ArcLifecycleTest.
func InstallArcLifecycleTestApps(ctx context.Context, a *arc.ARC, howMany int) error {
	// TODO: uninstall?
	testing.ContextLogf(ctx, "Installing %d copies of AndroidLifecycleTest app", howMany)
	for i := 0; i < howMany; i++ {
		if err := a.Install(ctx, androidLifecycleTestAPKPath(i)); err != nil {
			return errors.Wrapf(err, "failed to install %q", androidLifecycleTestAPKPath(i))
		}
	}
	testing.ContextLog(ctx, "Done")
	return nil
}
