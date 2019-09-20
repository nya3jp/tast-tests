// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TaskInfo contains the information found in TaskRecord. See:
// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/am/TaskRecord.java
type TaskInfo struct {
	// ID represents the TaskRecord ID.
	ID int
	// StackID represents the stack ID.
	StackID int
	// StackSize represents how many activities are in the stack.
	StackSize int
	// Bounds represents the task bounds in pixels. Caption is not taken into account.
	Bounds Rect
	// PkgName is the package name.
	PkgName string
	// ActivityName is the top-most activity name.
	ActivityName string

	// These properties are private since it is not clear whether they can be fetched using the Protobuf output.

	// windowState represents the window state.
	windowState WindowState
	// idle represents the activity idle state.
	// If the TaskRecord contains more than one activity, it refers to the top-most one.
	idle bool
	// resizable represents whether the activity is user-resizable or not.
	resizable bool
}

// DumpsysActivityActivities returns the "dumpsys activity activities" output as a list of TaskInfo.
func (a *ARC) DumpsysActivityActivities(ctx context.Context) ([]TaskInfo, error) {
	n, err := SDKVersion()
	if err != nil {
		return nil, err
	}
	switch n {
	case SDKN:
		return a.dumpsysActivityActivitiesN(ctx)
	case SDKP:
		return a.dumpsysActivityActivitiesP(ctx)
	default:
		return nil, errors.Errorf("unsupported Android version %d", n)
	}
}

// dumpsysActivityActivitiesN returns the "dumpsys activity activities" output as a list of TaskInfo.
// Should only be called on ARC NYC devices.
func (a *ARC) dumpsysActivityActivitiesN(ctx context.Context) (tasks []TaskInfo, err error) {
	// NYC doesn't support Probobuf output in dumpsys. Resorting to regexp.
	out, err := a.Command(ctx, "dumpsys", "activity", "activities").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not get 'dumpsys activity activities' output")
	}
	output := string(out)
	/*
		Looking for:
		Task id #2
		mFullscreen=false
		mBounds=Rect(0, 0 - 2400, 1600)
		mMinWidth=-1
		mMinHeight=-1
		mLastNonFullscreenBounds=Rect(0, 0 - 2400, 1600)
		* TaskRecord{cecb288 #2 A=com.android.settings U=0 StackId=2 sz=1}
			userId=0 effectiveUid=1000 mCallingUid=2000 mUserSetupComplete=true mCallingPackage=null
			affinity=com.android.settings
			intent={act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] flg=0x10000000 cmp=com.android.settings/.Settings}
			origActivity=com.android.settings/.Settings
			realActivity=com.android.settings/.Settings
			autoRemoveRecents=false isPersistable=true numFullscreen=1 taskType=0 mTaskToReturnTo=1
			rootWasReset=false mNeverRelinquishIdentity=true mReuseTask=false mLockTaskAuth=LOCK_TASK_AUTH_PINNABLE
			Activities=[ActivityRecord{388b75e u0 com.android.settings/.Settings t2}]
			askedCompatMode=false inRecents=true isAvailable=true
			lastThumbnail=null lastThumbnailFile=/data/system_ce/0/recent_images/2_task_thumbnail.png
			stackId=2
			hasBeenVisible=true mResizeMode=RESIZE_MODE_RESIZEABLE isResizeable=true firstActiveTime=1568651434414 lastActiveTime=1568651434414 (inactive for 0s)
			Arc Window State:
			mWindowState=WINDOW_STATE_MAXIMIZED mRestoreBounds=Rect(0, 0 - 0, 0)
			* Hist #0: ActivityRecord{388b75e u0 com.android.settings/.Settings t2}
				packageName=com.android.settings processName=com.android.settings
				[...] Abbreviated to save space
				state=RESUMED stopped=false delayedResume=false finishing=false
				keysPaused=false inHistory=true visible=true sleeping=false idle=true mStartingWindowState=STARTING_WINDOW_SHOWN
	*/
	regStr := `(?m)` + // Enable multiline.
		`^\s+Task id #(\d+)` + // Grab task id (group 1).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+mBounds=(?:(null)|Rect\((-?\d+),\s*(-?\d+)\s*-\s*(\d+),\s*(\d+)\))` + // Grab bounds or null bounds (groups 2-6).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*TaskRecord{.*StackId=(\d+)\s+sz=(\d*)}.*$` + // Grab stack Id (group 7) and stack size (group 8).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+realActivity=(.*)\/(.*)` + // Grab package name (group 9) and activity name (group 10).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+isResizeable=(\S+).*$` + // Grab window resizeablitiy (group 11).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+mWindowState=(\S+).*$` + // Window state (group 12)
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+ActivityRecord{.*` + // At least one ActivityRecord must be present.
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+idle=(\S+)` // Idle state (group 13).
	re := regexp.MustCompile(regStr)
	matches := re.FindAllStringSubmatch(string(output), -1)
	// At least it must match one activity. Home and/or Dummy activities must be present.
	if len(matches) == 0 {
		testing.ContextLog(ctx, "Using regexp: ", regStr)
		testing.ContextLog(ctx, "Output for regexp: ", string(output))
		return nil, errors.New("could not match any activity; regexp outdated perhaps?")
	}
	for _, groups := range matches {
		var t TaskInfo

		// On NYC some tasks could contain null bounds. They are represented with the default Rect value.
		if groups[2] != "null" {
			t.Bounds, err = parseBounds(groups[3:7])
			if err != nil {
				return nil, err
			}
		}

		for _, dst := range []struct {
			v     *int
			group int
		}{
			{&t.ID, 1},
			{&t.StackID, 7},
			{&t.StackSize, 8},
		} {
			*dst.v, err = strconv.Atoi(groups[dst.group])
			if err != nil {
				return nil, errors.Wrapf(err, "could not parse %q", groups[dst.group])
			}
		}
		t.PkgName = groups[9]
		t.ActivityName = groups[10]
		t.resizable, err = strconv.ParseBool(groups[11])
		if err != nil {
			return nil, err
		}
		// Taken from WindowPositioner.java, arc-nyc-mr1 branch:
		// http://cs/arc-nyc-mr1/frameworks/base/services/core/java/com/android/server/am/WindowPositioner.java
		ws := map[string]WindowState{
			"WINDOW_STATE_MAXIMIZED":         WindowStateMaximized,
			"WINDOW_STATE_FULLSCREEN":        WindowStateFullscreen,
			"WINDOW_STATE_NORMAL":            WindowStateNormal,
			"WINDOW_STATE_MINIMIZED":         WindowStateMinimized,
			"WINDOW_STATE_PRIMARY_SNAPPED":   WindowStatePrimarySnapped,
			"WINDOW_STATE_SECONDARY_SNAPPED": WindowStateSecondarySnapped,
		}
		val, ok := ws[groups[12]]
		if !ok {
			return nil, errors.Errorf("unsupported window state value: %q", groups[12])
		}
		t.windowState = val
		t.idle, err = strconv.ParseBool(groups[13])
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// dumpsysActivityActivitiesP returns the "dumpsys activity activities" output as a list of TaskInfo.
// Should only be called on ARC Pie devices.
func (a *ARC) dumpsysActivityActivitiesP(ctx context.Context) (tasks []TaskInfo, err error) {
	// TODO(crbug.com/989595): parse dumpsys protobuf output instead. Protobuf is supported by P, Q and R+.
	out, err := a.Command(ctx, "dumpsys", "activity", "activities").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not get 'dumpsys activity activities' output")
	}
	output := string(out)
	/*
		Looking for:
		Stack #2: type=standard mode=freeform
		isSleeping=false
		mBounds=Rect(0, 0 - 0, 0)
		  Task id #5
		  mBounds=Rect(1139, 359 - 1860, 1640)
		  mMinWidth=-1
		  mMinHeight=-1
		  mLastNonFullscreenBounds=Rect(1139, 359 - 1860, 1640)
		  * TaskRecordArc{TaskRecordArc{TaskRecord{54ef88b #5 A=com.android.settings.root U=0 StackId=2 sz=1}, WindowState{freeform restore-bounds=Rect(1139, 359 - 1860, 1640)}} , WindowState{freeform restore-bounds=Rect(1139, 359 - 1860, 1640)}}
			userId=0 effectiveUid=1000 mCallingUid=1000 mUserSetupComplete=true mCallingPackage=org.chromium.arc.applauncher
			affinity=com.android.settings.root
			intent={act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] flg=0x10210000 cmp=com.android.settings/.Settings}
			origActivity=com.android.settings/.Settings
			realActivity=com.android.settings/.Settings
			autoRemoveRecents=false isPersistable=true numFullscreen=1 activityType=1
			rootWasReset=true mNeverRelinquishIdentity=true mReuseTask=false mLockTaskAuth=LOCK_TASK_AUTH_PINNABLE
			Activities=[ActivityRecord{64b5e83 u0 com.android.settings/.Settings t5}]
			askedCompatMode=false inRecents=true isAvailable=true
			mRootProcess=ProcessRecord{8dc5d68 5809:com.android.settings/1000}
			stackId=2
			hasBeenVisible=true mResizeMode=RESIZE_MODE_RESIZEABLE_VIA_SDK_VERSION mSupportsPictureInPicture=false isResizeable=true lastActiveTime=1470240 (inactive for 4s)
			Arc Window State:
			mWindowMode=5 mRestoreBounds=Rect(1139, 359 - 1860, 1640) taskWindowState=0
		 * Hist #2: ActivityRecord{2f9c16c u0 com.android.settings/.SubSettings t8}
		     packageName=com.android.settings processName=com.android.settings
		     [...] Abbreviated to save space
		     state=RESUMED stopped=false delayedResume=false finishing=false
			 keysPaused=false inHistory=true visible=true sleeping=false idle=true mStartingWindowState=STARTING_WINDOW_NOT_SHOWN
	*/
	regStr := `(?m)` + // Enable multiline.
		`^\s+Task id #(\d+)` + // Grab task id (group 1).
		`\s+mBounds=Rect\((-?\d+),\s*(-?\d+)\s*-\s*(\d+),\s*(\d+)\)` + // Grab bounds (groups 2-5).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*TaskRecord{.*StackId=(\d+)\s+sz=(\d*)}.*$` + // Grab stack Id (group 6) and stack size (group 7).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+realActivity=(.*)\/(.*)` + // Grab package name (group 8) and activity name (group 9).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+isResizeable=(\S+).*$` + // Grab window resizeablitiy (group 10).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+mWindowMode=\d+.*taskWindowState=(\d+).*$` + // Grab window state (group 11).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+ActivityRecord{.*` + // At least one ActivityRecord must be present.
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+idle=(\S+)` // Idle state (group 12).
	re := regexp.MustCompile(regStr)
	matches := re.FindAllStringSubmatch(string(output), -1)
	// At least it must match one activity. Home and/or Dummy activities must be present.
	if len(matches) == 0 {
		testing.ContextLog(ctx, "Using regexp: ", regStr)
		testing.ContextLog(ctx, "Output for regexp: ", string(output))
		return nil, errors.New("could not match any activity; regexp outdated perhaps?")
	}
	for _, groups := range matches {
		var t TaskInfo
		var windowState int
		t.Bounds, err = parseBounds(groups[2:6])
		if err != nil {
			return nil, err
		}

		for _, dst := range []struct {
			v     *int
			group int
		}{
			{&t.ID, 1},
			{&t.StackID, 6},
			{&t.StackSize, 7},
			{&windowState, 11},
		} {
			*dst.v, err = strconv.Atoi(groups[dst.group])
			if err != nil {
				return nil, errors.Wrapf(err, "could not parse %q", groups[dst.group])
			}
		}
		t.PkgName = groups[8]
		t.ActivityName = groups[9]
		t.resizable, err = strconv.ParseBool(groups[10])
		if err != nil {
			return nil, err
		}
		t.idle, err = strconv.ParseBool(groups[12])
		if err != nil {
			return nil, err
		}
		t.windowState = WindowState(windowState)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// Helper functions.

// parseBounds returns a Rect by parsing a slice of 4 strings.
// Each string represents the left, top, right and bottom values, in that order.
func parseBounds(s []string) (bounds Rect, err error) {
	if len(s) != 4 {
		return Rect{}, errors.Errorf("expecting a slice of length 4, got %d", len(s))
	}
	var right, bottom int
	for i, dst := range []*int{&bounds.Left, &bounds.Top, &right, &bottom} {
		*dst, err = strconv.Atoi(s[i])
		if err != nil {
			return Rect{}, errors.Wrapf(err, "could not parse %q", s[i])
		}
	}
	bounds.Width = right - bounds.Left
	bounds.Height = bottom - bounds.Top
	return bounds, nil
}
