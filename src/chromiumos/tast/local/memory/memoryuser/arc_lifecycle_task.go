// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/testexec"
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
	// E.g. "org.chromium.arc.testapp.lifecycle00/org.chromium.arc.testapp.lifecycle.ALLOC".
	return fmt.Sprintf("%s%02d/%s.MainActivity", arcLifecycleTestPkg, t.id, arcLifecycleTestPkg)
}

func (t *ArcLifecycleTask) intentAction(action string) string {
	// E.g. "org.chromium.arc.testapp.lifecycle00.ALLOC".
	return fmt.Sprintf("%s%02d.%s", arcLifecycleTestPkg, t.id, action)
}

var errAllocationNotDone = errors.New("allocation is not done")

var amStartRE = regexp.MustCompile("(?m)^Status: ok$")

// Run starts the AndroidLifecycleTest app, and uses it to allocate memory.
func (t *ArcLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	// Dump the activity stack.
	// if stackOut, err := testEnv.arc.Command(ctx, "am", "stack", "list").Output(testexec.DumpLogOnError); err != nil {
	// 	testing.ContextLog(ctx, "Failed to dump activity stacks: ", err)
	// } else {
	// 	testing.ContextLog(ctx, "Activity stacks: ", string(stackOut))
	// }
	// Dump processes that are running.
	//adb shell ps
	if psOut, err := testEnv.arc.Command(ctx, "ps").Output(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed to ps: ", err)
	} else {
		testing.ContextLog(ctx, "ps")
		for _, line := range strings.Split(string(psOut), "\n") {
			if strings.Contains(line, "lifecycle") {
				testing.ContextLog(ctx, line)
			}
		}
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
	tokenString := time.Now().Format(time.RFC3339)
	startOut, err := testEnv.arc.Command(
		ctx,
		"am", "start", "-W", "-n", t.mainActivity(),
		"--el", "size", sizeString,
		"--ef", "ratio", ratioString,
		"--es", "token", tokenString,
	).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to start %q", t.mainActivity())
	}
	if !amStartRE.MatchString(string(startOut)) {
		return errors.Errorf("failed to start AndroidLifecycleTask, output %q", string(startOut))
	}

	// Wait for allocation to complete.
	allocCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// NB: We use tokenString to make sure we don't match a previous run of the same app.
	doneString := fmt.Sprintf("lifecycle%02d: allocation complete %s", t.id, tokenString)

	// NB: Wrap the WaitForLogcat in a Poll because logcat sometimes exits
	// early.
	if err := testing.Poll(allocCtx, func(context.Context) error {
		return testEnv.arc.WaitForLogcat(allocCtx, func(line string) bool {
			return strings.Contains(line, doneString)
		})
	}, &testing.PollOptions{}); err != nil {
		return errors.Wrap(err, "failed to wait for allocation complete")
	}

	// Minimize
	// TODO: remove once bug that stops unminimized apps from being killed my lmk is fixed
	findWindow := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		Name:      "Main Activity",
		ClassName: "RootView",
	}
	window := uig.Find(findWindow)
	minimize := window.Find(ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Minimize",
		ClassName: "FrameCaptionButton",
	})
	if err := uig.Do(ctx, testEnv.tconn, uig.Steps(
		minimize.LeftClick(),
		uig.WaitUntilDescendantGone(findWindow, 10*time.Second),
	)); err != nil {
		return errors.Wrap(err, "failed to minimize App")
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

// StillAlive checks to see if the app is still running.
func (t *ArcLifecycleTask) StillAlive(_ context.Context, _ *TestEnv) bool {
	// TODO: use ps or something
	return false
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

// StillAlive returns false if the launch failed, or true if app is still
// responding to intents.
func (t *BestEffortArcLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	if !t.succeeded {
		// The app launch might have timed out, so we didn't send the allocation
		// intent, but still be running and responding to intents now. So don't
		// report that the app is StillAlive if the launch+allocations didn't
		// succeed.
		return false
	}
	return t.ArcLifecycleTask.StillAlive(ctx, testEnv)
}

// NewBestEffortArcLifecycleTask creates a BestEffort MemoryTask with the same
// parameters as NewArcLifecycleTask above.
func NewBestEffortArcLifecycleTask(id int, allocateBytes int64, ratio float64, limit memory.Limit) *BestEffortArcLifecycleTask {
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
