// Copyright 2019 The ChromiumOS Authors
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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
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
	Bounds coords.Rect
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
	// focused represents whether the activity's window has focus or not.
	focused bool
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

	regStrForActivitiesP = `ActivityRecord{[0-9a-fA-F]* u[0-9]* ([^,]*)\/([^,]*) t[0-9]*(?: f)?}`
)

var (
	regExpP              = regexp.MustCompile(regStrP)
	regExpForActivitiesP = regexp.MustCompile(regStrForActivitiesP)
)

// TaskInfosFromDumpsys returns a list of all available TaskInfo from the "dumpsys activity activities" and "dumpsys Wayland" if needed.
func (a *ARC) TaskInfosFromDumpsys(ctx context.Context) ([]TaskInfo, error) {
	n, err := SDKVersion()
	if err != nil {
		return nil, err
	}
	switch n {
	case SDKP:
		return a.dumpsysActivityActivitiesP(ctx)
	case SDKR, SDKS, SDKT:
		tasks, err := a.dumpsysActivityActivitiesR(ctx)

		if err != nil {
			return nil, errors.Wrap(err, "could not get info from 'dumpsys activity activities'")
		}
		// We here use dumpsysWaylandR to fulfil the windowState property which is not available in R's "dumpsys activity activities"
		tasksFromWayland, err := a.dumpsysWaylandR(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "could not get info from 'dumpsys Wayland'")
		}
		fillWindowState(tasks, tasksFromWayland)
		return tasks, nil
	default:
		return nil, errors.Errorf("unsupported Android version %d", n)
	}
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
	// At least it must match one activity. Home and/or Fake activities must be present.
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

// dumpsysActivityActivitiesR returns the "dumpsys activity activities" output as a list of TaskInfo.
// Should only be called on ARC R devices.
func (a *ARC) dumpsysActivityActivitiesR(ctx context.Context) (tasks []TaskInfo, err error) {
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

	// dumpsys returns a tree of window containers which we are to traverse in the following.
	// For each TaskProto node in the tree, we create a TaskInfo item representing it, then add an ActivityInfo for each *immediate* child ActivityRecord node.
	// (Note that TaskProto can be nested, but each ActivityRecord is only associated to its immediate parent.)
	// TODO(b/152576355): Task stack info are not provided by TaskProto, we need to use other sources for the information.

	// Holds the focused activity while traversing the windowing hierarchy.
	var focusedActivity string

	// Helper to represent all window container types.
	type windowContainer interface {
		GetWindowContainer() *server.WindowContainerProto
	}

	// Forward declaration for recursive call.
	var traverse func(windowContainer)

	traverse = func(wc windowContainer) {
		if d, ok := wc.(*server.DisplayContentProto); ok && d != nil {
			focusedActivity = d.GetFocusedApp()
			// Since Android 12(S), the `WindowContainer` field has been deprecated and `Task` only exist within `RootDisplayArea`,
			// instead of both in `RootDisplayArea` and `DisplayArea`. Therefore, we also need to traverse RootDisplayArea to extract `Task`.
			if rda := d.GetRootDisplayArea(); rda != nil {
				traverse(rda)
			}
		}

		if t, ok := wc.(*server.TaskProto); ok && t != nil {
			b := t.GetBounds()
			var title string
			if t.GetResumedActivity() != nil {
				title = t.GetResumedActivity().GetTitle()
			}
			ti := TaskInfo{
				ID:      int(t.GetId()),
				Bounds:  coords.NewRectLTRB(int(b.GetLeft()), int(b.GetTop()), int(b.GetRight()), int(b.GetBottom())),
				resumed: t.GetResumedActivity() != nil,
				// android.content.pm.ActivityInfo.RESIZE_MODE_UNRESIZEABLE == 0
				resizable: t.GetResizeMode() != 0,
				focused:   title == focusedActivity,
				// TODO(b/152576355): StackID, StackSize
			}

			// Add all immediate ActivityRecord children.
			// `TaskFragment` is introduced since Android 12(S) in `Task` as the container for `Activity` or `TaskFragment`.
			// Therefore, when `TaskFragment` exists, we need to extract activities from `TaskFragment` instead of `Task`,
			// and to traverse `TaskFragment` to get the child tasks.
			var ac windowContainer
			if t.GetTaskFragment() != nil {
				ac = t.GetTaskFragment()
				traverse(ac)
			} else {
				ac = t
			}
			for _, c := range ac.GetWindowContainer().GetChildren() {
				a := c.GetActivity()
				if a == nil {
					continue
				}
				// Neither package name or activity name are allowed to use "/". Testing for "len != 2" is safe.
				s := strings.Split(a.GetName(), "/")
				if len(s) != 2 {
					// Name is either a component name/string (eg: "com.android.settings/.FallbackHome") or a window title ("NavigationBar").
					// As we need both package and activity name, we just skip this activity if it has the latter format.
					testing.ContextLog(ctx, "Skipping this activity as its title doesn't have the format <package name>/<activity name>: ", a.GetName())
					continue
				}
				ti.ActivityInfos = append(ti.ActivityInfos, ActivityInfo{s[0], s[1]})
			}

			tasks = append(tasks, ti)
		}

		// Traverse all child containers. A node can only contain either of these types, others will give nil.
		// We don't need to check nil as nil nodes simply return no children and terminate the recursion.
		for _, c := range wc.GetWindowContainer().GetChildren() {
			traverse(c)
			traverse(c.GetDisplayArea())
			traverse(c.GetDisplayContent())
			traverse(c.GetTask())
			traverse(c.GetTaskFragment())
			traverse(c.GetWindow())
			traverse(c.GetWindowToken())
		}
	}

	traverse(am.GetRootWindowContainer())

	return tasks, nil
}

// dumpsysWaylandR returns the "dumpsys Wayland" output as a list of TaskInfo, which is complementary to dumpsysActivityActivitiesR.
// Should only be called on ARC R devices.
func (a *ARC) dumpsysWaylandR(ctx context.Context) (tasks []TaskInfo, err error) {
	out, err := a.Command(ctx, "dumpsys", "Wayland").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "could not get 'dumpsys Wayland")
	}
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^\s+Task Container 0x[[:alnum:]]+\s+` + // Match Task Container section.
		`\(task\s+(\d+).+\n` + // Grab the taskID (group 1).
		`.*window state:\s+([A-Z_]+)`) // Grab window state (group 2).
	const (
		taskIDIndex      = 1
		windowStateIndex = 2
	)

	matches := re.FindAllStringSubmatch(string(out), -1)
	wsMap := map[string]WindowState{
		"WINDOW_STATE_NORMAL":            WindowStateNormal,
		"WINDOW_STATE_MAXIMIZED":         WindowStateMaximized,
		"WINDOW_STATE_FULLSCREEN":        WindowStateFullscreen,
		"WINDOW_STATE_MINIMIZED":         WindowStateMinimized,
		"WINDOW_STATE_PRIMARY_SNAPPED":   WindowStatePrimarySnapped,
		"WINDOW_STATE_SECONDARY_SNAPPED": WindowStateSecondarySnapped,
		"WINDOW_STATE_PIP":               WindowStatePIP,
	}
	for _, groups := range matches {
		ws, ok := wsMap[groups[windowStateIndex]]
		if !ok {
			return nil, errors.Errorf("unsupported window state value: %q", groups[windowStateIndex])
		}
		taskID, err := strconv.Atoi(groups[taskIDIndex])
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse taskID: %q", groups[taskIDIndex])
		}
		ti := TaskInfo{
			ID:          taskID,
			windowState: ws,
		}

		tasks = append(tasks, ti)
	}

	return tasks, nil
}

// Helper functions.

// fillWindowState fills windowState property from tasks dumped from Wayland. This is necessary for ARC++ R or above because
// "dumpsys activity actvities" doesn't have them.
func fillWindowState(tasks, tasksFromWayland []TaskInfo) {
	for i := range tasks {
		for _, taskFromWayland := range tasksFromWayland {
			if tasks[i].ID == taskFromWayland.ID {
				tasks[i].windowState = taskFromWayland.windowState
				break
			}
		}
	}
}

// parseBounds returns a Rect by parsing a slice of 4 strings.
// Each string represents the left, top, right and bottom values, in that order.
func parseBounds(s []string) (bounds coords.Rect, err error) {
	if len(s) != 4 {
		return coords.Rect{}, errors.Errorf("expecting a slice of length 4, got %d", len(s))
	}
	var right, bottom int
	for i, dst := range []*int{&bounds.Left, &bounds.Top, &right, &bottom} {
		*dst, err = strconv.Atoi(s[i])
		if err != nil {
			return coords.Rect{}, errors.Wrapf(err, "could not parse %q", s[i])
		}
	}
	bounds.Width = right - bounds.Left
	bounds.Height = bottom - bounds.Top
	return bounds, nil
}

// MeminfoAppSummary holds the summary metrics for a single app from dumpsys
// meminfo.
type MeminfoAppSummary struct {
	// PSS metrics. PSS is the size of all the pages mapped into a process
	// divided by each page's reference count. When adding values from
	JavaHeapPssKb     int32
	NativeHeapPssKb   int32
	CodePssKb         int32
	StackPssKb        int32
	GraphicsPssKb     int32
	PrivateOtherPssKb int32
	SystemPssKb       int32

	// RSS metrics. RSS is the size of all the pages mapped into a process.
	JavaHeapRssKb   int32
	NativeHeapRssKb int32
	CodeRssKb       int32
	StackRssKb      int32
	GraphicsRssKb   int32
	UnknownRssKb    int32
}

// DumpsysMeminfoPackage returns summary metrics for a specific app package.
func (a *ARC) DumpsysMeminfoPackage(ctx context.Context, pkg string) (*MeminfoAppSummary, error) {
	n, err := SDKVersion()
	if err != nil {
		return nil, err
	}
	switch n {
	case SDKP, SDKR, SDKS, SDKT:
		return a.dumpsysMeminfoPackageR(ctx, pkg)
	default:
		return nil, errors.Errorf("unsupported Android version %d", n)
	}
}

func (a *ARC) dumpsysMeminfoPackageR(ctx context.Context, pkg string) (*MeminfoAppSummary, error) {
	output, err := a.Command(ctx, "dumpsys", "meminfo", "--proto", pkg).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get 'dumpsys meminfo --proto %s' output", pkg)
	}

	mi := &server.MemInfoDumpProto{}
	if err := proto.Unmarshal(output, mi); err != nil {
		if dir, ok := testing.ContextOutDir(ctx); !ok {
			testing.ContextLog(ctx, "Failed to save protobuf message. Could not get ContextOutDir()")
		} else if f, err := ioutil.TempFile(dir, "meminfo-protobuf-message-*.bin"); err != nil {
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
	if len(mi.AppProcesses) != 1 {
		return nil, errors.Errorf("failed to parse meminfo, expected 1 AppProcess, got %d", len(mi.AppProcesses))
	}
	app := mi.AppProcesses[0]
	if app == nil {
		return nil, errors.New("failed to parse meminfo, missing AppProcesses[0]")
	}
	process := app.ProcessMemory
	if process == nil {
		return nil, errors.New("failed to parse meminfo, missing AppProcesses[0].ProcessMemory")
	}
	summary := process.AppSummary
	if summary == nil {
		return nil, errors.New("failed to parse meminfo, missing AppProcesses[0].ProcessMemory.AppSummary")
	}
	return &MeminfoAppSummary{
		JavaHeapPssKb:     summary.GetJavaHeapPssKb(),
		NativeHeapPssKb:   summary.GetNativeHeapPssKb(),
		CodePssKb:         summary.GetCodePssKb(),
		StackPssKb:        summary.GetStackPssKb(),
		GraphicsPssKb:     summary.GetGraphicsPssKb(),
		PrivateOtherPssKb: summary.GetPrivateOtherPssKb(),
		SystemPssKb:       summary.GetSystemPssKb(),
		JavaHeapRssKb:     summary.GetJavaHeapRssKb(),
		NativeHeapRssKb:   summary.GetNativeHeapRssKb(),
		CodeRssKb:         summary.GetCodeRssKb(),
		StackRssKb:        summary.GetStackRssKb(),
		GraphicsRssKb:     summary.GetGraphicsRssKb(),
		UnknownRssKb:      summary.GetUnknownRssKb(),
	}, nil
}
