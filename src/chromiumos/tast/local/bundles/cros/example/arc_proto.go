// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"android.com/frameworks/base/core/proto/android/server"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcProto,
		Desc:         "Sample test to parse the probuf output from ARC dumpsys",
		Contacts:     []string{"tast-owners@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_all", "chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"ArcPipTastTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func ArcProto(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const apkName = "ArcPipTastTest.apk"
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	const pkgName = "org.chromium.arc.testapp.pictureinpicture"
	actPIP, err := arc.NewActivity(a, pkgName, ".MaPipBaseActivity")
	if err != nil {
		s.Fatal("Failed creating Activity: ", err)
	}
	defer actPIP.Close()
	if err := actPIP.Start(ctx); err != nil {
		s.Fatal("Failed starting Activity: ", err)
	}

	if err := uiClick(ctx, d, ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)Launch PIP Activity")); err != nil {
		s.Fatal("Failed to press button: ", err)
	}

	if err := uiClick(ctx, d, ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)Enter PIP")); err != nil {
		s.Fatal("Failed to press button: ", err)
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed waiting: ", err)
	}

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	defer act.Stop(ctx)

	taskInfo, err := a.DumpsysActivityActivities(ctx)
	if err != nil {
		s.Fatal("Failed to dumpsys: ", err)
	}
	s.Log("TaskInfo: ", taskInfo)

	if err := dumpWindows(ctx, s, a); err != nil {
		s.Fatal("Failed to dumpWindows: ", err)
	}

	if err := dumpActivities(ctx, s, a); err != nil {
		s.Fatal("Failed to dumpActivities")
	}
}

func dumpWindows(ctx context.Context, s *testing.State, a *arc.ARC) error {
	cmd1 := a.Command(ctx, "dumpsys", "window", "--proto")
	output1, err := cmd1.Output()
	if err != nil {
		return err
	}

	state := &server.WindowManagerServiceDumpProto{}
	if err := proto.Unmarshal(output1, state); err != nil {
		return err
	}
	root := state.RootWindowContainer

	for _, d := range root.GetDisplays() {
		for _, s := range d.GetStacks() {
			for _, t := range s.GetTasks() {
				for _, a := range t.GetAppWindowTokens() {
					testing.ContextLog(ctx, "Name: ", a.GetName())
					for _, w := range a.GetWindowToken().GetWindows() {
						testing.ContextLog(ctx, "Title: ", w.GetIdentifier().GetTitle())
						testing.ContextLog(ctx, "Frame: ", w.GetFrame())
						testing.ContextLog(ctx, "DisplayFrame: ", w.GetDisplayFrame())
						testing.ContextLog(ctx, "DisplayId: ", w.GetDisplayId())
						testing.ContextLog(ctx, "StackId: ", w.GetStackId())

						f := w.GetFrame()
						r := arc.Rect{
							Left:   int(f.GetLeft()),
							Top:    int(f.GetTop()),
							Width:  int(f.GetRight() - f.GetLeft()),
							Height: int(f.GetBottom() - f.GetTop()),
						}
						testing.ContextLogf(ctx, "Frame is: %+v", r)
					}
				}
			}
		}
	}
	return nil
}

func dumpActivities(ctx context.Context, s *testing.State, a *arc.ARC) error {

	// The contents of protobuf (--proto option) vs. the text (non-proto) output might be a bit different.
	// It might be possible that some data is present in the protobuf output but not in the text, and vice-versa.
	// The easiest way to know what is in the protobuf output is by looking at .proto files.
	// For example, the structure "dumpsys activity --proto activities" can be found here:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/core/proto/android/server/activitymanagerservice.proto
	cmd := a.Command(ctx, "dumpsys", "activity", "--proto", "activities")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// For example, to enumerate all running activities, you should do:

	// 1) Unmarshall the protobuf data.
	am := &server.ActivityManagerServiceDumpActivitiesProto{}
	if err := proto.Unmarshal(output, am); err != nil {
		return err
	}

	// 2) Just fetch the data that you need. In this case the activities.
	super := am.GetActivityStackSupervisor()
	for _, d := range super.GetDisplays() {
		s.Log("Display Id: ", d.GetId())
		for _, stack := range d.GetStacks() {
			s.Log("->Stack Id: ", stack.GetId())
			for _, t := range stack.GetTasks() {
				s.Log("-->Task Id: ", t.GetId())

				s.Log("-->Task real activity: ", t.GetRealActivity())
				for _, a := range t.GetActivities() {
					id := a.GetIdentifier()
					// Either a component name/string (eg: "com.android.settings/.FallbackHome")
					// or a window title ("NavigationBar").
					testing.ContextLogf(ctx, "---> Title: %v, state= %v", id.GetTitle(), a.GetState())
				}
			}
		}
	}
	return nil
}

// uiClick sends a "Click" message to an UI Object.
// The UI Object is selected from opts, which are the selectors.
func uiClick(ctx context.Context, d *ui.Device, opts ...ui.SelectorOption) error {
	obj := d.Object(opts...)
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "could not click on widget")
	}
	return nil
}
