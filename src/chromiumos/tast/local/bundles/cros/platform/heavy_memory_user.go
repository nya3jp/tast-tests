// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeavyMemoryUser,
		Desc:         "Tests heavy memory use with Chrome, ARC and VMs running",
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      10 * time.Minute,
		Data:         []string{"NOVALegacy-1.1.5.apk"},
		SoftwareDeps: []string{"android", "chrome_login", "vm_host"},
	})
}

func HeavyMemoryUser(ctx context.Context, s *testing.State) {
	const (
		// This starts the NOVA Legacy game
		apk          = "NOVALegacy-1.1.5.apk"
		pkg          = "com.gameloft.android.ANMP.GloftNOHM"
		activityName = ".MainActivity"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	arcFunc := func(a *arc.ARC, d *ui.Device) {
		must(d.Object(ui.ClassName("android.view.View"), ui.PackageName(pkg)).WaitForExists(ctx))
		time.After(60 * time.Second)
	}

	aTask := memoryuser.AndroidTask{Apk: apk, Pkg: pkg, ActivityName: activityName, TestFunc: arcFunc}

	urls := []string{
		"https://drive.google.com",
		"https://photos.google.com",
		"https://news.google.com",
		"https://plus.google.com",
		"https://maps.google.com",
		"https://play.google.com/store",
		"https://play.google.com/music",
		"https://youtube.com",
		"https://www.nytimes.com",
		"https://www.whitehouse.gov",
		"https://www.wsj.com",
		"https://washingtonpost.com",
		"https://www.foxnews.com",
		"https://www.nbc.com",
		"https://www.amazon.com",
		"https://www.cnn.com",
	}

	cTask := memoryuser.ChromeTask{Urls: urls, NumTabs: 75}

	vmCommandArgs := memoryuser.VMCmd{"dd", "if=/dev/urandom", "of=/mnt/stateful/lxd_conf/foo", "bs=3M", "count=1K"}
	vmCommands := []memoryuser.VMCmd{vmCommandArgs, vmCommandArgs}
	vmTask := memoryuser.VMTask{VMCommands: vmCommands}

	memoryuser.RunMemoryUserTest(ctx, s, aTask, cTask, vmTask)
}
