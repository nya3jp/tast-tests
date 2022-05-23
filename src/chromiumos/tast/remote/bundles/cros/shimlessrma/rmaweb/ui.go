// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rmaweb contains web-related common functions used in the Shimless RMA app.
package rmaweb

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
)

const timeInSecondToLoadPage = 30
const timeinSecondToEnableButton = 5
const longTimeInSecondToEnableButton = 60
const manuallyDisableWP = "Manually turn off"
const rsuDisableWP = "Perform RMA Server Unlock (RSU)"

// WaitForRebootStart indicates the time to wait before reboot starting.
const WaitForRebootStart = 10 * time.Second

// UIHelper holds the resources required to communicate with Shimless RMA App.
type UIHelper struct {
	// Client contains Shimless RMA App client.
	Client         pb.AppServiceClient
	Dut            *dut.DUT
	FirmwareHelper *firmware.Helper
	RPCClient      *rpc.Client
}

// NewUIHelper creates UIHelper.
func NewUIHelper(ctx context.Context, dut *dut.DUT, firmwareHelper *firmware.Helper, rpcHint *testing.RPCHint, key string, reconnect bool) (*UIHelper, error) {
	cl, client, err := createShimlessClient(ctx, dut, firmwareHelper, rpcHint, key, reconnect)
	if err != nil {
		return nil, err
	}
	uiHelper := &UIHelper{client, dut, firmwareHelper, cl}
	return uiHelper, nil
}

// DisposeResource will close the resources which are required in UIHelper.
func (uiHelper *UIHelper) DisposeResource(cleanupCtx context.Context) {
	uiHelper.RPCClient.Close(cleanupCtx)
	uiHelper.Client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})
}

// WelcomePageOperation handles all operations on Welcome Page.
func (uiHelper *UIHelper) WelcomePageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Chromebook repair", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Get started", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// ComponentsPageOperation handles all operations on Components Selection Page.
func (uiHelper *UIHelper) ComponentsPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Select which components were replaced", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Base Accelerometer"); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next"); err != nil {
		return err
	}

	return nil
}

// OwnerPageOperation handles all operations on Owner Selection Page.
func (uiHelper *UIHelper) OwnerPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "After repair, who will be using the device?", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, "Device will go to the same user"); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectPageChooseRSU handles all operations on WP Page and select RSU.
func (uiHelper *UIHelper) WriteProtectPageChooseRSU(ctx context.Context) error {
	return uiHelper.writeProtectPageOperation(ctx, rsuDisableWP)
}

// WriteProtectPageChooseManul handles all operations on WP Page and select manual option.
func (uiHelper *UIHelper) WriteProtectPageChooseManul(ctx context.Context) error {
	if err := uiHelper.writeProtectPageOperation(ctx, manuallyDisableWP); err != nil {
		return err
	}
	if err := uiHelper.changeWriteProtectStatus(ctx, servo.FWWPStateOff); err != nil {
		return err
	}
	return uiHelper.FirmwareHelper.Servo.RunCR50Command(ctx, "bpforce disconnect atboot")

}

// WipeDevicePageOperation handles all operations on wipe device Page.
func (uiHelper *UIHelper) WipeDevicePageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Device is going to the same user. Erase user data?", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, "Erase all data"); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectDisabledPageOperation handles all operations on Write Protect Disabled Page.
func (uiHelper *UIHelper) WriteProtectDisabledPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Write Protect is turned off", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next"); err != nil {
		return err
	}

	return nil
}

// WriteProtectEnabledPageOperation handles all operations on Write Protect Enable Page.
func (uiHelper *UIHelper) WriteProtectEnabledPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Manually enable write-protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next"); err != nil {
		return err
	}

	return nil
}

// FirmwareInstallationPageOperation handles all operations on Firmware Installation Page.
func (uiHelper *UIHelper) FirmwareInstallationPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Install firmware image", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// DeviceInformationPageOperation handles all operations on device information Page.
func (uiHelper *UIHelper) DeviceInformationPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Please confirm device information", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next"); err != nil {
		return err
	}

	return nil
}

// DeviceProvisionPageOperation handles all operations on device provisioning Page.
func (uiHelper *UIHelper) DeviceProvisionPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Provisioning the device…", timeInSecondToLoadPage); err != nil {
		return err
	}

	return nil
}

// CalibratePageOperation handles all operations on calibrate Page.
func (uiHelper *UIHelper) CalibratePageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Prepare to calibrate device components", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// FinalizingRepairPageOperation handles all operations on finalizing repair Page.
func (uiHelper *UIHelper) FinalizingRepairPageOperation(ctx context.Context) error {
	// Firstly, reset battery signal & turn on WP.
	if err := uiHelper.FirmwareHelper.Servo.RunCR50Command(ctx, "bpforce follow_batt_pres atboot"); err != nil {
		return err
	}
	if err := uiHelper.changeWriteProtectStatus(ctx, servo.FWWPStateOn); err != nil {
		return err
	}

	if err := uiHelper.waitForPageToLoad(ctx, "Finalizing repair", timeInSecondToLoadPage); err != nil {
		return err
	}

	return nil
}

// RepairCompeletedPageOperation handles all operations on repair completed Page.
func (uiHelper *UIHelper) RepairCompeletedPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Repair is complete", longTimeInSecondToEnableButton); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Reboot"); err != nil {
		return err
	}

	return nil
}

// RSUPageOperation handles all operations on RSU Page.
func (uiHelper *UIHelper) RSUPageOperation(ctx context.Context) error {
	// Change battery status and WP status
	if err := uiHelper.FirmwareHelper.Servo.RunCR50Command(ctx, "bpforce disconnect atboot"); err != nil {
		return err
	}
	if err := uiHelper.changeWriteProtectStatus(ctx, servo.FWWPStateOff); err != nil {
		return err
	}

	if err := uiHelper.waitForPageToLoad(ctx, "Perform RMA Server Unlock", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickLink(ctx, "this URL"); err != nil {
		return err
	}
	url, err := uiHelper.retrieveTextByPrefix(ctx, "https")
	if err != nil {
		return err
	}
	// Extract the parameter from url.
	challengeCode := uiHelper.parseChallengeCode(url)

	output, err := uiHelper.Dut.Conn().CommandContext(ctx, "/usr/local/bin/rma_reset", "-c", challengeCode).Output()
	if err != nil {
		return err
	}
	authCode := uiHelper.parseAuthCode(string(output))

	if err := uiHelper.clickButton(ctx, "Done"); err != nil {
		return err
	}
	if err := uiHelper.enterIntoTextInput(ctx, authCode, "Enter the 8-character unlock code"); err != nil {
		return err
	}
	testing.Sleep(ctx, time.Second*5)
	if err := uiHelper.clickButton(ctx, "Next"); err != nil {
		return err
	}

	return nil
}

// BypassFirmwareInstallation will skip firmware installation.
func (uiHelper *UIHelper) BypassFirmwareInstallation(ctx context.Context) error {
	// This sleep is important since we need to wait for RMAD to update state file completed.
	testing.Sleep(ctx, 3*time.Second)
	// Add "firmware_updated":true to state file.
	if _, err := uiHelper.Dut.Conn().CommandContext(ctx, "sed", "-i", fmt.Sprintf("s/%s/%s/g", ".$", ",\"firmware_updated\":true}"), "/mnt/stateful_partition/unencrypted/rma-data/state").Output(); err != nil {
		return err
	}

	if err := uiHelper.Dut.Reboot(ctx); err != nil {
		return err
	}

	return nil
}

// OpenCCDIfNecessary opens CCD if CCD is not open yet.
func (uiHelper *UIHelper) OpenCCDIfNecessary(ctx context.Context) error {
	if val, err := uiHelper.FirmwareHelper.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
		return err
	} else if val != servo.Open {
		if err := uiHelper.FirmwareHelper.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
			return err
		}
	}

	return nil
}

// SetupInitStatus setup initial status for shimless testing.
func (uiHelper *UIHelper) SetupInitStatus(ctx context.Context) error {
	// If error is raised, then Factory is already disabled.
	// Therefore, ignore any error.
	uiHelper.changeFactoryMode(ctx, "disable")
	// Open CCD needs to be executed after disable Factory mode.
	// It is because disable Factory mode will also lock CCD.
	if err := uiHelper.OpenCCDIfNecessary(ctx); err != nil {
		return err
	}
	if err := uiHelper.changeWriteProtectStatus(ctx, servo.FWWPStateOn); err != nil {
		return err
	}

	return nil
}

func createShimlessClient(ctx context.Context, dut *dut.DUT, firmwareHelper *firmware.Helper, rpcHint *testing.RPCHint, key string, reconnect bool) (*rpc.Client, pb.AppServiceClient, error) {
	if err := firmwareHelper.WaitConnect(ctx); err != nil {
		return nil, nil, err
	}

	// Setup rpc.
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return nil, nil, err
	}

	request := &pb.NewShimlessRMARequest{
		ManifestKey: key,
		Reconnect:   reconnect,
	}
	client := pb.NewAppServiceClient(cl.Conn)
	if _, err := client.NewShimlessRMA(ctx, request, grpc.WaitForReady(true)); err != nil {
		return nil, nil, err
	}

	return cl, client, nil
}

func (uiHelper *UIHelper) changeWriteProtectStatus(ctx context.Context, status servo.FWWPStateValue) error {
	err := uiHelper.FirmwareHelper.Servo.SetFWWPState(ctx, status)
	return err
}

func (uiHelper *UIHelper) changeFactoryMode(ctx context.Context, status string) error {
	_, err := uiHelper.Dut.Conn().CommandContext(ctx, "gsctool", "-aF", status).Output()
	return err
}

func (uiHelper *UIHelper) writeProtectPageOperation(ctx context.Context, radioButtonLabel string) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Select how you would like to turn off Write Protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, radioButtonLabel); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

func (uiHelper *UIHelper) waitAndClickButton(ctx context.Context, label string, timeInSecond int32) error {
	if _, err := uiHelper.Client.WaitUntilButtonEnabled(ctx, &pb.WaitUntilButtonEnabledRequest{
		Label:            label,
		DurationInSecond: timeInSecond,
	}); err != nil {
		return err
	}

	return uiHelper.clickButton(ctx, label)
}

func (uiHelper *UIHelper) clickButton(ctx context.Context, label string) error {
	_, err := uiHelper.Client.LeftClickButton(ctx, &pb.LeftClickButtonRequest{
		Label: label,
	})
	return err
}

func (uiHelper *UIHelper) waitForPageToLoad(ctx context.Context, title string, timeInSecond int32) error {
	_, err := uiHelper.Client.WaitForPageToLoad(ctx, &pb.WaitForPageToLoadRequest{
		Title:            title,
		DurationInSecond: timeInSecond,
	})
	return err
}

func (uiHelper *UIHelper) clickRadioButton(ctx context.Context, label string) error {
	_, err := uiHelper.Client.LeftClickRadioButton(ctx, &pb.LeftClickRadioButtonRequest{
		Label: label,
	})

	return err
}

func (uiHelper *UIHelper) clickLink(ctx context.Context, label string) error {
	if _, err := uiHelper.Client.LeftClickLink(ctx, &pb.LeftClickLinkRequest{
		Label: label,
	}); err != nil {
		return err
	}

	return nil
}

func (uiHelper *UIHelper) retrieveTextByPrefix(ctx context.Context, prefix string) (string, error) {
	res, err := uiHelper.Client.RetrieveTextByPrefix(ctx, &pb.RetrieveTextByPrefixRequest{
		Prefix: prefix,
	})
	if err != nil {
		return "", err
	}

	return res.Value, nil
}

func (uiHelper *UIHelper) parseChallengeCode(url string) string {
	re := regexp.MustCompile("challenge=(.*)&")
	return re.FindStringSubmatch(url)[1]
}

func (uiHelper *UIHelper) parseAuthCode(raw string) string {
	re := regexp.MustCompile(`Authcode:\s+([a-zA-Z0-9]*)`)
	return re.FindStringSubmatch(raw)[1]
}

func (uiHelper *UIHelper) enterIntoTextInput(ctx context.Context, content, textInputName string) error {
	_, err := uiHelper.Client.EnterIntoTextInput(ctx, &pb.EnterIntoTextInputRequest{
		TextInputName: textInputName,
		Content:       content,
	})

	if err != nil {
		return err
	}
	return nil
}
