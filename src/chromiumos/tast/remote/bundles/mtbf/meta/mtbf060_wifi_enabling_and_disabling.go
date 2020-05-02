// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF060WifiEnablingAndDisabling,
		Desc:     "Enable/Disable WiFi with ARC++ Test Ap",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive060DUTArcEnableWifi(ctx context.Context, dut *utils.Device) error {
	setTrue := "Calling WifiManager.setWifiEnabled(true)"
	setSuccess := "setWifiEnabled returned with result: true"
	jobName := "log1"

	dut.EnterToAppAndVerify(ctx, ".MainActivity", "com.example.abhishekbh.wificlient", "text=WifiClient")
	dut.Client.ExecCommand(dut.DeviceID, "logcat -c").Do(ctx, service.Sleep(0))
	dut.Client.ExecCommand(dut.DeviceID, "logcat").Async(true).Alias(jobName).Do(ctx, service.Sleep(time.Second*10))
	dut.Client.UIAClick(dut.DeviceID).Selector("text=SETWIFIENABLEDSTATE TRUE").Do(ctx, service.Sleep(time.Second*10))

	logName := "/tmp/mtbfstg2-logcat1.txt"
	dut.Client.StopCommand(jobName).IsReturn(true).Output(logName).Do(ctx)

	b1, _ := dut.Client.GetValueFromLog(logName, setTrue).Do(ctx)
	b2, _ := dut.Client.GetValueFromLog(logName, setSuccess).Do(ctx)

	if !b1 || !b2 {
		return mtbferrors.New(mtbferrors.EnableWifi, nil)
	}

	return nil
}

func drive060DUTDisableWifi(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060DisableWifi"); err != nil {
		return err
	}

	return nil
}

func drive060DUTVerifyWifiEnabled(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060VerifyWifi.enabled"); err != nil {
		return err
	}

	return nil
}

func drive060DUTVerifyWifiDisabled(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060VerifyWifi.disabled"); err != nil {
		return err
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

	logName := "/tmp/mtbfstg2-logcat2.txt"
	dut.Client.StopCommand(jobName).IsReturn(true).Output(logName).Do(ctx)

	b1, _ := dut.Client.GetValueFromLog(logName, setFalse).Do(ctx)
	b2, _ := dut.Client.GetValueFromLog(logName, setSuccess).Do(ctx)

	if !b1 || !b2 {
		return mtbferrors.New(mtbferrors.DisableWifi, nil)
	}

	return nil
}

func cleanup060DUTArc(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.Client.ExecCommand(dut.DeviceID, "shell am force-stop com.example.abhishekbh.wificlient").Do(ctx)
}

func MTBF060WifiEnablingAndDisabling(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF060WifiEnablingAndDisabling",
		Description: "Enable/Disable WiFi with ARC++ Test Ap",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Disable DUT Wifi by cdputils")
		if mtbferr := drive060DUTDisableWifi(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Enable DUT Wifi by ARC app")
		if mtbferr := drive060DUTArcEnableWifi(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Verify DUT Wifi enabled")
		if mtbferr := drive060DUTVerifyWifiEnabled(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Disable DUT Wifi by ARC app")
		if mtbferr := drive060DUTArcDisableWifi(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Verify DUT Wifi disabled")
		if mtbferr := drive060DUTVerifyWifiDisabled(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup060DUTArc(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
