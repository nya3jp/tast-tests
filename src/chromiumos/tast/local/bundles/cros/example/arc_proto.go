// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
	pb "frameworks/base/core/proto/android/server"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcProto,
		Desc:         "Sample test to parse the probuf output from ARC dumpsys",
		Contacts:     []string{"tast-owners@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
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
	am := &pb.ActivityManagerServiceDumpActivitiesProto{}
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
