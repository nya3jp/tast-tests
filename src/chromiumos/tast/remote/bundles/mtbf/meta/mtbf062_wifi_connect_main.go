// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"sync"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF062WifiConnectMain,
		Desc:         "Remote main case of TC61 802.11n Support. It will call local case MTBF062WifiConnectin DUT",
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars:         []string{"dut.id", "allion.api.server", "allion.deviceId", "detach.status.server"},
	})
}

//MTBF062WifiConnectMain Remote main case of TC61 802.11n Support. It will call local case MTBF062WifiConnectin DUT
func MTBF062WifiConnectMain(ctx context.Context, s *testing.State) {
	s.Log("Start to run MTBF062WifiConnect sub case")
	localTestName := "wifi.MTBF062WifiConnect"
	dutID := common.GetVar(ctx, s, "dut.id")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	deviceID := common.GetVar(ctx, s, "allion.deviceId")
	allionServerURL := common.GetVar(ctx, s, "allion.api.server")
	defer common.EnableEthernet(ctx, s, allionServerURL, deviceID)
	// IMPORTANT: set a right detachDuration
	flags := common.GetDetachedFlags(s, 300)
	s.Log("Step 1: start main test case")

	// 2. Calling tast to run local test in DUT
	var wg sync.WaitGroup
	wg.Add(1)

	go func(waitGroup *sync.WaitGroup) {
		defer waitGroup.Done()
		s.Log("Step 2: Start calling tast to run a local test")

		if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, localTestName); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Step 2: End calling tast to run a local test")
	}(&wg)

	s.Log("Step 3: Polling the status server to check if sub case finished")

	if err := common.PollDetachedCaseDone(ctx, s, detachStatusSvr, dutID, localTestName); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIDtchPollSubCase, err, "MTBF062"))
	}

	common.EnableEthernet(ctx, s, allionServerURL, deviceID)
	wg.Wait()
	s.Log("End running mtbf sub-tests")
}
