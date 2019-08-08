// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"strings"
	"time"

	"android.com/frameworks/base/core/proto/android/server"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/local/arc"
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
		Timeout:      4 * time.Minute,
	})
}

func ArcProto(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

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

	cmd1 := a.Command(ctx, "dumpsys", "window", "--proto")
	output1, err := cmd1.Output()
	if err != nil {
		s.Fatal("Failed to launch dumpsys: ", err)
	}

	state := &server.WindowManagerServiceDumpProto{}
	if err := proto.Unmarshal(output1, state); err != nil {
		s.Fatal("Failed to launch dumpsys: ", err)
	}
	root := state.RootWindowContainer

	for _, d := range root.GetDisplays() {
		for _, s := range d.GetStacks() {
			for _, t := range s.GetTasks() {
				for _, a := range t.GetAppWindowTokens() {
					testing.ContextLog(ctx, "Name: ", a.GetName())
					if a.GetName() == act.PackageName()+"/"+".Settings" {
						for _, w := range a.GetWindowToken().GetWindows() {
							if strings.HasPrefix(w.GetIdentifier().GetTitle(), "Splash Screen") {
								continue
							}
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
	}

	// The contents of protobuf (--proto option) vs. the text (non-proto) output might be a bit different.
	// It might be possible that some data is present in the protobuf output but not in the text, and vice-versa.
	// The easiest way to know what is in the protobuf output is by looking at .proto files.
	// For example, the structure "dumpsys activity --proto activities" can be found here:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/core/proto/android/server/activitymanagerservice.proto
	cmd := a.Command(ctx, "dumpsys", "activity", "--proto", "activities")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to launch dumpsys: ", err)
	}

	// For example, to enumerate all running activities, you should do:

	// 1) Unmarshall the protobuf data.
	am := &server.ActivityManagerServiceDumpActivitiesProto{}
	if err := proto.Unmarshal(output, am); err != nil {
		s.Fatal("Failed to parse activity manager: ", err)
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
			}
		}
	}
}
