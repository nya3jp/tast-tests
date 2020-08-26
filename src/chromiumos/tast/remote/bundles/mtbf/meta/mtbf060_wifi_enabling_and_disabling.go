// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/service"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF060WifiEnablingAndDisabling,
		Desc:         "Enable/Disable WiFi with ARC++ Test Ap",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.requestURL"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.wifi.WifiService",
			"tast.mtbf.svc.CommService",
		},
	})
}

func MTBF060WifiEnablingAndDisabling(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF060WifiEnablingAndDisabling",
		Description: "Enable/Disable WiFi with ARC++ Test Ap",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		wsc := wifi.NewWifiServiceClient(cl.Conn)

		s.Log("Disable DUT Wifi by tast.mtbf.wifi.WifiService")
		if _, mtbferr := wsc.Disable(ctx, &empty.Empty{}); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Enable DUT Wifi by ARC app")
		if mtbferr := drive060DUTArcEnableWifi(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Verify DUT Wifi enabled")
		if _, mtbferr := wsc.Verify(ctx, &wifi.VerifyRequest{Enable: true}); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Disable DUT Wifi by ARC app")
		if mtbferr := drive060DUTArcDisableWifi(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Verify DUT Wifi disabled")
		if _, mtbferr := wsc.Verify(ctx, &wifi.VerifyRequest{Enable: false}); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)

		s.Log("Start case cleanup")

		client.Comments("Recover env").Do(ctx)
		client.ExecCommand(dutID, "shell am force-stop com.example.abhishekbh.wificlient").Do(ctx)

		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

func drive060DUTArcEnableWifi(ctx context.Context, dut *utils.Device) error {
	setTrue := "Calling WifiManager.setWifiEnabled(true)"
	setSuccess := "setWifiEnabled returned with result: true"
	jobName := "log1"

	dut.EnterToAppAndVerify(ctx, ".MainActivity", "com.example.abhishekbh.wificlient", "text=WifiClient")
	dut.Client.ExecCommand(dut.DeviceID, "logcat -c").Do(ctx, service.Sleep(0))
	dut.Client.ExecCommand(dut.DeviceID, "logcat").Async(true).Alias(jobName).Do(ctx, service.Sleep(time.Second*10))
	dut.Client.UIAClick(dut.DeviceID).Selector("text=SETWIFIENABLEDSTATE TRUE").Do(ctx, service.Sleep(time.Second*10))

	logName := "/tmp/logcat1.txt"
	dut.Client.StopCommand(jobName).IsReturn(true).Output(logName).Do(ctx)

	b1, _ := dut.Client.GetValueFromLog(logName, setTrue).Do(ctx)
	b2, _ := dut.Client.GetValueFromLog(logName, setSuccess).Do(ctx)

	if !b1 || !b2 {
		return mtbferrors.New(mtbferrors.EnableWifi, nil)
	}

	return nil
}

func drive060DUTArcDisableWifi(ctx context.Context, dut *utils.Device) error {
	setFalse := "Calling WifiManager.setWifiEnabled(false)"
	setSuccess := "setWifiEnabled returned with result: true"
	jobName := "log2"

	dut.EnterToAppAndVerify(ctx, ".MainActivity", "com.example.abhishekbh.wificlient", "text=WifiClient")
	dut.Client.ExecCommand(dut.DeviceID, "logcat -c").Do(ctx, service.Sleep(0))
	dut.Client.ExecCommand(dut.DeviceID, "logcat").Async(true).Alias(jobName).Do(ctx)

	dut.Client.UIAClick(dut.DeviceID).Selector("text=SETWIFIENABLEDSTATE FALSE").Do(ctx)

	logName := "/tmp/logcat2.txt"
	dut.Client.StopCommand(jobName).IsReturn(true).Output(logName).Do(ctx)

	b1, _ := dut.Client.GetValueFromLog(logName, setFalse).Do(ctx)
	b2, _ := dut.Client.GetValueFromLog(logName, setSuccess).Do(ctx)

	if !b1 || !b2 {
		return mtbferrors.New(mtbferrors.DisableWifi, nil)
	}

	return nil
}
