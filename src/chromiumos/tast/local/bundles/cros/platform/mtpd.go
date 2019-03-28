// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Mtpd,
		Desc: "Verifies mtpd is running and responds to D-Bus calls",
		Contacts: []string{
			"amistry@chromium.org",
			"benchan@chromium.org",
			"chromeos-files-app@google.com",
		},
	})
}

func Mtpd(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.Mtpd"
		dbusPath      = "/org/chromium/Mtpd"
		dbusInterface = "org.chromium.Mtpd"

		job = "mtpd"
	)

	s.Log("Restarting mtpd service and waiting for D-Bus service")
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatal("Failed to restart mtpd: ", err)
	}

	_, dbusObj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatalf("Failed to connect to D-Bus service %s: %v", dbusName, err)
	}

	var result bool
	if err := dbusObj.CallWithContext(ctx, dbusInterface+".IsAlive", 0).Store(&result); err != nil {
		s.Error("Failed to call IsAlive D-Bus method: ", err)
	} else if !result {
		s.Error("Unexpected false result from IsAlive")
	}
}
