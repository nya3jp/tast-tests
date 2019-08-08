// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	pb "chromiumos/aosp_proto"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Proto,
		Desc:         "Sample test to manipulate an app with UI automator",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"todo-mvp.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func Proto(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	cmd := a.Command(ctx, "dumpsys", "activity", "activities", "--proto")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to launch dumpsys: ", err)
	}

	am := &pb.ActivityManagerServiceDumpActivitiesProto{}
	if err := proto.Unmarshal(output, am); err != nil {
		s.Fatal("Failed to parse activity manager:", err)
	}
	s.Logf("ActivityManager: %+v", *am)
}
