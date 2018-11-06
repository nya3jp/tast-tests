// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeavyMemoryUser,
		Desc:         "Tests heavy memory use with Chrome, ARC and VMs running",
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"android", "chrome_login", "vm_host"},
	})
}

func HeavyMemoryUser(ctx context.Context, s *testing.State) {
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

	cTask := memoryuser.ChromeTask{Urls: urls, NumTabs: 50}

	vmCmd := memoryuser.VMCmd{"dd", "if=/dev/urandom", "of=/mnt/stateful/lxd_conf/foo", "bs=3M", "count=1K"}
	vmCommands := []memoryuser.VMCmd{vmCmd, vmCmd}
	vmTask := memoryuser.VMTask{VMCommands: vmCommands}
	memTasks := []memoryuser.MemoryTask{cTask, vmTask}
	memoryuser.RunMemoryUserTest(ctx, s, memTasks)
}
