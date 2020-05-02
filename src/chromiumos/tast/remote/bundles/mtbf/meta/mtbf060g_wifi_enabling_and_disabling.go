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
		Func:     MTBF060GWifiEnablingAndDisabling,
		Desc:     "Enable/Disable WiFi with ARC++ Test Ap",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive060GDUTArcEnableWifi(ctx context.Context, dut *utils.Device) error {
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

func drive060GDUTDisableWifi(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060DisableWifi"); err != nil {
		return err
	}

	return nil
}

func drive060GDUTVerifyWifiEnabled(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060VerifyWifi.enabled"); err != nil {
		return err
	}

	return nil
}

func drive060GDUTVerifyWifiDisabled(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "wifi.MTBF060VerifyWifi.disabled"); err != nil {
		return err
	}

	return nil
}

func drive060GDUTArcDisableWifi(ctx context.Context, dut *utils.Device) error {
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

func cleanup060GDUTArc(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.Client.ExecCommand(dut.DeviceID, "shell am force-stop com.example.abhishekbh.wificlient").Do(ctx)
}

func MTBF060GWifiEnablingAndDisabling(ctx context.Context, s *testing.State) {
	dutID, err := s.DUT().GetARCDeviceID(ctx)
	if err != nil {
		s.Fatal(mtbferrors.OSNoArcDeviceID, err)
	}

	addr, err := common.CatsNodeAddress(ctx, s)
	if err != nil {
		s.Fatal("Failed to get cats node addr: ", err)
	}

	androidTest, err := sdk.New(addr)
	if err != nil {
		s.Fatal("Failed to new androi test: ", err)
	}

	if err := common.CatsMTBFLogin(ctx, s); err != nil {
		s.Fatal("Failed to do MTBFLogin: ", err)
	}

	report, _, err := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        "case_name",
		Description: "A new case",
		ReportPath:  "report/path",
		DutID:       dutID,
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		if err := drive060GDUTDisableWifi(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive060GDUTArcEnableWifi(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive060GDUTVerifyWifiEnabled(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive060GDUTArcDisableWifi(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive060GDUTVerifyWifiDisabled(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup060GDUTArc(ctx, dutDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
