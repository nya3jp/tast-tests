// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Dlcservice,
		Desc: "Launches dlcservice to verify dlcservice exits on idle and can accept API calls.",
		Attr: []string{"informational"},
	})
}

func Dlcservice(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.DlcService"
		dbusPath      = "/org/chromium/DlcService"
		dbusInterface = "org.chromium.DlcServiceInterface"

		job = "dlcservice"
	)

	s.Logf("Restarting %s job", job)
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to start %s: %v", job, err)
	}

	if err := upstart.WaitForJobStatus(ctx, job, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s failed to arrive status: %v.", job, err)
	} else {
		s.Logf("job %s stopped.", job)
	}

	s.Logf("Asking dlcservice for installed DLC modules.")
	conn, _ := dbus.SystemBus()
	obj := conn.Object(dbusName, dbusPath)
	var dlcModuleListStr string
	if err := obj.CallWithContext(ctx, dbusInterface+".GetInstalled", 0).Store(&dlcModuleListStr); err != nil {
		s.Errorf("Failed to get installed DLC modules: %v", err)
	} else {
		s.Logf("Return string is %q", dlcModuleListStr)
	}

	if err := upstart.WaitForJobStatus(ctx, job, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s failed to arrive status: %v.", job, err)
	} else {
		s.Logf("job %s stopped.", job)
	}
}
