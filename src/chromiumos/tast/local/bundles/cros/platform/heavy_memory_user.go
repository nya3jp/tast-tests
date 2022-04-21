// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeavyMemoryUser,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests heavy memory use with Chrome, ARC and VMs running",
		Contacts:     []string{"asavery@chromium.org", "chromeos-storage@google.com"},
		// TODO(http://b/172075721): Test is disabled until it can be fixed
		// Attr:         []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func HeavyMemoryUser(ctx context.Context, s *testing.State) {
	urls := []string{
		"https://drive.google.com",
		"https://photos.google.com",
		"https://news.google.com",
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
	cTask := memoryuser.ChromeTask{URLs: urls, NumTabs: 50}

	vmCmd := memoryuser.VMCmd{"dd", "if=/dev/urandom", "of=foo", "bs=3M", "count=1K"}
	vmCommands := []memoryuser.VMCmd{vmCmd, vmCmd}
	vmTask := memoryuser.VMTask{Cmds: vmCommands}

	rp := &memoryuser.RunParameters{
		ParallelTasks: true,
	}
	memTasks := []memoryuser.MemoryTask{&cTask, &vmTask}
	if err := memoryuser.RunTest(ctx, s.OutDir(), memTasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
