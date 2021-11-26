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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// ArcLifecycleUnit launches an Android app which allocates memory.
type ArcLifecycleUnit struct {
	id            int
	allocateBytes int64
	ratio         float64
	limit         memory.Limit
	minimize      bool
}

// ArcLifecycleUnitCount is the number of different ARC lifecycle app packages,
// which is the number of different lifecycle apps we can run at the same time.
const ArcLifecycleUnitCount = 100

const arcLifecycleTestPkg = "org.chromium.arc.testapp.lifecycle"

func (t *ArcLifecycleUnit) packageName() string {
	// E.g. "org.chromium.arc.testapp.lifecycle00".
	return fmt.Sprintf("%s%02d", arcLifecycleTestPkg, t.id)
}

func (t *ArcLifecycleUnit) mainActivity() string {
	// E.g. "org.chromium.arc.testapp.lifecycle00/org.chromium.arc.testapp.lifecycle.MainActivity".
	return fmt.Sprintf("%s/%s.MainActivity", t.packageName(), arcLifecycleTestPkg)
}

func (t *ArcLifecycleUnit) intentAction(action string) string {
	// E.g. "org.chromium.arc.testapp.lifecycle00.ALLOC".
	return fmt.Sprintf("%s.%s", t.packageName(), action)
}

var amStartRE = regexp.MustCompile("(?m)^Status: ok$")

// Run starts the AndroidLifecycleTest app, and uses it to allocate memory.
func (t *ArcLifecycleUnit) Run(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn) error {
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
	startOut, err := a.Command(
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
		done, err := t.doneAllocating(ctx, a)
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

	if t.minimize {
		minimizeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := tconn.Call(minimizeCtx, nil, `async (packageName) => {
			const getWindows = tast.promisify(chrome.autotestPrivate.getAppWindowList);
			const setState = tast.promisify(chrome.autotestPrivate.setAppWindowState);
			for (const w of await getWindows()) {
				if (w.arcPackageName === packageName) {
					await setState(w.id, {eventType: 'WMEventMinimize'});
				}
			}
		}`, t.packageName()); err != nil {
			return errors.Wrapf(err, "failed to minimize window for %q", t.packageName())
		}
	}

	return nil
}

// Close kills the AndroidLifecycleTest app.
func (t *ArcLifecycleUnit) Close(ctx context.Context, a *arc.ARC) {
	err := a.Command(ctx, "am", "force-stop", t.packageName()).Run(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to Close %q: %v", t.packageName(), err)
	}
}

func (t *ArcLifecycleUnit) doneAllocating(ctx context.Context, arc *arc.ARC) (bool, error) {
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

// StillAlive checks to see if the AndroidLifecycleTest app is alive and
// responding to intents.
func (t *ArcLifecycleUnit) StillAlive(ctx context.Context, a *arc.ARC) bool {
	_, err := t.doneAllocating(ctx, a)
	return err == nil
}

// NewArcLifecycleUnit creates a helper to allocate memory inside an app in ARC.
//  id            - Which copy of ArcLifecycleTest app to use.
//  allocateBytes - mow much memory to allocate.
//  ratio         - the compression ratio of allocated memory.
//  limit         - if not nil, wait for Limit after allocation.
//  minimize      - minimize the app window after allocation completes.
func NewArcLifecycleUnit(id int, allocateBytes int64, ratio float64, limit memory.Limit, minimize bool) *ArcLifecycleUnit {
	return &ArcLifecycleUnit{id, allocateBytes, ratio, limit, minimize}
}

// FillArcMemory installs, launches, and allocated in ArcLifecycleTest apps
// until one is killed, filling up memory in ARC.
func FillArcMemory(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, unitBytes int64, ratio float64) (func(context.Context) error, error) {
	var units []*ArcLifecycleUnit
	cleanup := func(ctx context.Context) error {
		var res error
		for _, unit := range units {
			// Uninstalling the APK will also kill the app, if it's running.
			if err := a.Uninstall(ctx, unit.packageName()); err != nil {
				testing.ContextLogf(ctx, "Failed to uninstall %q: %s", unit.packageName(), err)
				if res == nil {
					res = err
				}
			}
		}
		return res
	}
	for i := 0; i < ArcLifecycleUnitCount; i++ {
		if err := installAndroidLifecycleTestAPK(ctx, a, i); err != nil {
			return cleanup, err
		}
		unit := NewArcLifecycleUnit(i, unitBytes, ratio, nil, false)
		units = append(units, unit)
		if err := unit.Run(ctx, a, tconn); err != nil {
			return cleanup, errors.Wrapf(err, "failed to run ArcLifecycleUnit %d", i)
		}
		for _, unit := range units {
			if !unit.StillAlive(ctx, a) {
				testing.ContextLogf(ctx, "FillArcMemory started %d units of %d MiB before first kill", len(units), unitBytes/memory.MiB)
				return cleanup, nil
			}
		}
	}
	return cleanup, errors.Errorf("started %d ArcLifecycleUnits, but all are still alive", ArcLifecycleUnitCount)
}

// ArcLifecycleTask wraps ArcLifecycleUnit to conform to the MemoryTask and
// KillableTask interfaces.
type ArcLifecycleTask struct {
	ArcLifecycleUnit
}

// ArcLifecycleTask is a MemoryTask.
var _ MemoryTask = (*ArcLifecycleTask)(nil)

// ArcLifecycleTask is a KillableTask.
var _ KillableTask = (*ArcLifecycleTask)(nil)

// String returns a friendly string containing the
func (t *ArcLifecycleTask) String() string {
	return fmt.Sprintf("ARC lifecycle%02d", t.id)
}

// NeedVM returns false, because we don't need Crostini.
func (t *ArcLifecycleTask) NeedVM() bool {
	return false
}

// Run starts the AndroidLifecycleTest app, and uses it to allocate memory.
func (t *ArcLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	return t.ArcLifecycleUnit.Run(ctx, testEnv.arc, testEnv.tconn)
}

// Close kills the AndroidLifecycleTest app.
func (t *ArcLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {
	t.ArcLifecycleUnit.Close(ctx, testEnv.arc)
}

// StillAlive checks to see if the AndroidLifecycleTest app is alive and
// responding to intents.
func (t *ArcLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	return t.ArcLifecycleUnit.StillAlive(ctx, testEnv.arc)
}

// NewArcLifecycleTask creates a MemoryTask that runs the ArcLifecycleTest app
// and uses it to allocate memory.
//  id            - Which copy of ArcLifecycleTest app to use.
//  allocateBytes - mow much memory to allocate.
//  ratio         - the compression ratio of allocated memory.
//  limit         - if not nil, wait for Limit after allocation.
//  minimize      - minimize the app window after allocation completes.
func NewArcLifecycleTask(id int, allocateBytes int64, ratio float64, limit memory.Limit, minimize bool) *ArcLifecycleTask {
	return &ArcLifecycleTask{*NewArcLifecycleUnit(id, allocateBytes, ratio, limit, minimize)}
}

func androidLifecycleTestAPKPath(id int) string {
	return fmt.Sprintf("/usr/local/libexec/tast/apks/local/cros/ArcLifecycleTest%02d.apk", id)
}

func installAndroidLifecycleTestAPK(ctx context.Context, a *arc.ARC, id int) error {
	const installRetries = 3
	var err error
	for i := 0; ; i++ {
		err = a.Install(ctx, androidLifecycleTestAPKPath(id))
		if err == nil {
			return nil
		}
		if i >= installRetries {
			return errors.Wrapf(err, "failed to install ArcLifecycleTest%02d", id)
		}
		testing.ContextLogf(ctx, "Failed to install ArcLifecycleTest%02d on try %d: %s", id, i, err)
	}
}

// InstallArcLifecycleTestApps installs 'howMany' copies of ArcLifecycleTest.
// The maximum number of apps that can be installed is ArcLifecycleUnitCount.
func InstallArcLifecycleTestApps(ctx context.Context, a *arc.ARC, num int) error {
	if num > ArcLifecycleUnitCount {
		return errors.Errorf("requested %d number of apps to install, which is higher than the max of %d", num, ArcLifecycleUnitCount)
	}
	testing.ContextLogf(ctx, "Installing %d copies of AndroidLifecycleTest app", num)
	for i := 0; i < num; i++ {
		if err := installAndroidLifecycleTestAPK(ctx, a, i); err != nil {
			return err
		}
	}
	testing.ContextLog(ctx, "Done")
	return nil
}
