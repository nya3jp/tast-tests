// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"android.com/frameworks/base/core/proto/android/server"
	"github.com/golang/protobuf/proto"

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
	// ActivityInfos is the activities in the task
	ActivityInfos []ActivityInfo

	// These properties are private since it is not clear whether they can be fetched using the Protobuf output.

	// windowState represents the window state.d
	windowState WindowState
	// resumed represents the activity resumed state.
	// If the TaskRecord contains more than one activity, it refers to the top-most one.
	resumed bool
	// resizable represents whether the activity is user-resizable or not.
	resizable bool
}

// ActivityInfo contains the information found in ActivityRecord
type ActivityInfo struct {
	// PackageName is the package name.
	PackageName string
	// ActivityName is the name of the activity.
	ActivityName string
}

const (
	/*
		Regular Expression for parsing the output of dumpsys in N.

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
	regStrN string = `(?m)` + // Enable multiline.
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

	/*
		Regular Expression for parsing the output of dumpsys in P.

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
	regStrP = `(?m)` + // Enable multiline.
		`^\s+Task id #(\d+)` + // Grab task id (group 1).
		`\s+mBounds=Rect\((-?\d+),\s*(-?\d+)\s*-\s*(\d+),\s*(\d+)\)` + // Grab bounds (groups 2-5).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*TaskRecord{.*StackId=(\d+)\s+sz=(\d*)}.*$` + // Grab stack Id (group 6) and stack size (group 7).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+realActivity=(.*)\/(.*)` + // Grab package name (group 8) and activity name (group 9).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+Activities=\[(.*)\]` + // A list of activities (group 10).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+isResizeable=(\S+).*$` + // Grab window resizeablitiy (group 11).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+mWindowMode=\d+.*taskWindowState=(\d+).*$` + // Grab window state (group 12).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*\s+idle=(\S+)` // Idle state (group 13).

	regStrForActivitiesP = `ActivityRecord{[0-9a-fA-F]* u[0-9]* ([^,]*)\/([^,]*) t[0-9]*}`
)

var (
	regExpN              = regexp.MustCompile(regStrN)
	regExpP              = regexp.MustCompile(regStrP)
	regExpForActivitiesP = regexp.MustCompile(regStrForActivitiesP)
)

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
	case SDKQ:
		return a.dumpsysActivityActivitiesQ(ctx)
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
	matches := regExpN.FindAllStringSubmatch(string(output), -1)
	// At least it must match one activity. Home and/or Dummy activities must be present.
	if len(matches) == 0 {
		testing.ContextLog(ctx, "Using regexp: ", regStrN)
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
		// TODO(crbug/1024139): Parse all the activities in the task.
		t.ActivityInfos = append(t.ActivityInfos, ActivityInfo{groups[9], groups[10]})
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
		t.resumed, err = strconv.ParseBool(groups[13])
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
	matches := regExpP.FindAllStringSubmatch(string(output), -1)
	// At least it must match one activity. Home and/or Dummy activities must be present.
	if len(matches) == 0 {
		testing.ContextLog(ctx, "Using regexp: ", regStrP)
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

		// TODO(takise): Use SubexpNames to avoid hard coding the indexes.
		for _, dst := range []struct {
			v     *int
			group int
		}{
			{&t.ID, 1},
			{&t.StackID, 6},
			{&t.StackSize, 7},
			{&windowState, 12},
		} {
			*dst.v, err = strconv.Atoi(groups[dst.group])
			if err != nil {
				return nil, errors.Wrapf(err, "could not parse %q", groups[dst.group])
			}
		}
		t.resizable, err = strconv.ParseBool(groups[11])
		if err != nil {
			return nil, err
		}
		t.resumed, err = strconv.ParseBool(groups[13])
		if err != nil {
			return nil, err
		}
		t.windowState = WindowState(windowState)
		matchesForActivities := regExpForActivitiesP.FindAllStringSubmatch(groups[10], -1)
		if len(matchesForActivities) == 0 {
			testing.ContextLog(ctx, "Using regexp: ", regStrForActivitiesP)
			testing.ContextLog(ctx, "Test string for regexp: ", groups[10])
			return nil, errors.New("could not match any activity; regexp outdated perhaps?")
		}
		for _, activityGroups := range matchesForActivities {
			t.ActivityInfos = append(t.ActivityInfos, ActivityInfo{activityGroups[1], activityGroups[2]})
		}

		tasks = append(tasks, t)
	}
	return tasks, nil
}

// dumpsysActivityActivitiesQ returns the "dumpsys activity activities" output as a list of TaskInfo.
// Should only be called on ARC Q devices.
func (a *ARC) dumpsysActivityActivitiesQ(ctx context.Context) (tasks []TaskInfo, err error) {
	output, err := a.Command(ctx, "dumpsys", "activity", "--proto", "activities").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not get 'dumpsys activity --proto activities' output")
	}

	am := &server.ActivityManagerServiceDumpActivitiesProto{}
	if err := proto.Unmarshal(output, am); err != nil {
		if dir, ok := testing.ContextOutDir(ctx); !ok {
			testing.ContextLog(ctx, "Failed to save protobuf message. Could not get ContextOutDir()")
		} else if f, err := ioutil.TempFile(dir, "activity-activities-protobuf-message-*.bin"); err != nil {
			testing.ContextLog(ctx, "Failed to save protobuf message. Could not create temp file: ", err)
		} else {
			defer f.Close()
			if _, err := f.Write(output); err != nil {
				testing.ContextLog(ctx, "Failed to save protobuf message. Could not write to file: ", err)
			} else {
				testing.ContextLogf(ctx, "Protobuf message saved in test out directory. Filename: %q", path.Base(f.Name()))
			}
		}
		return nil, errors.Wrap(err, "failed to parse activity manager protobuf")
	}

	super := am.GetActivityStackSupervisor()
	for _, d := range super.GetDisplays() {
		for _, stack := range d.GetStacks() {
			for _, t := range stack.GetTasks() {
				ti := TaskInfo{}
				ti.ID = int(t.GetId())
				ti.StackID = int(stack.GetId())
				ti.StackSize = len(stack.GetTasks())
				// Some special activities like arc.Dummy and arc.Home don't have the OrigActivity.
				// In those cases, just get the RealActivity.
				name := t.GetOrigActivity()
				if name == "" {
					name = t.GetRealActivity()
				}
				for _, a := range t.GetActivities() {
					id := a.GetIdentifier()
					// Neither package name or activity name are allowed to use "/". Testing for "len != 2" is safe.
					s := strings.Split(name, "/")
					if len(s) != 2 {
						// id is either a component name/string (eg: "com.android.settings/.FallbackHome") or a window title ("NavigationBar").
						// As we need both package and activity name, we just skip this activity if it has the latter format.
						testing.ContextLog(ctx, "Skipping this activity as its title doesn't have the format <package name>/<activity name>: ", id.GetTitle())
						continue
					}
					ti.ActivityInfos = append(ti.ActivityInfos, ActivityInfo{s[0], s[1]})
				}
				b := t.GetBounds()
				ti.Bounds = Rect{
					Left:   int(b.GetLeft()),
					Top:    int(b.GetTop()),
					Width:  int(b.GetRight() - b.GetLeft()),
					Height: int(b.GetBottom() - b.GetTop())}
				// Any value different than 0 (RESIZE_MODE_UNRESIZEABLE) means it is resizable. Defined in ActivityInfo.java. See:
				// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/java/android/content/pm/ActivityInfo.java
				ti.resizable = t.GetResizeMode() != 0

				conf := t.GetConfigurationContainer().GetMergedOverrideConfiguration()
				winConf := conf.GetWindowConfiguration()
				wm := winConf.GetWindowingMode()
				// Windowing mode constants taken from WindowConfiguration.java. See:
				// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/java/android/app/WindowConfiguration.java
				// TODO(crbug.com/1005422) Minimized, Maximized and PIP modes are not supported. Find a replacement.
				// WINDOWING_MODE_PINNED is an acceptable temporay substitute for PIP.
				ws := map[int32]WindowState{
					1: WindowStateFullscreen,
					2: WindowStatePIP, // WINDOWING_MODE_PINNED
					3: WindowStatePrimarySnapped,
					4: WindowStateSecondarySnapped,
					5: WindowStateNormal,
				}
				val, ok := ws[wm]
				if !ok {
					return nil, errors.Errorf("unsupported window state value: %d", ws)
				}
				ti.windowState = val

				// TODO(crbug.com/1005422): Protobuf output does not provide "resumed" information. Find a replacement.
				ti.resumed = false

				tasks = append(tasks, ti)
			}
		}
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
