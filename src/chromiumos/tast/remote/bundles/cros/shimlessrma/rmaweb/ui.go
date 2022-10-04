// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rmaweb contains web-related common functions used in the Shimless RMA app.
package rmaweb

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
)

// DestinationOption indicates destination.
type DestinationOption string

// WriteProtectDisableOption indicates write protection disabling approach.
type WriteProtectDisableOption string

const (
	// SameUser indicates devices goes to same user.
	SameUser DestinationOption = "SAME_USER"

	// DifferentUser indicates devices goes to different user.
	DifferentUser DestinationOption = "DIFFERENT_USER"

	// Manual indicates using battery disconnection to disable write protect.
	Manual WriteProtectDisableOption = "MANUAL"

	// Rsu indicates using rsu to disable write protect.
	Rsu WriteProtectDisableOption = "RSU"

	// WaitForRebootStart indicates the time to wait before reboot starting.
	WaitForRebootStart = 10 * time.Second

	timeInSecondToLoadPage         = 30
	timeInSecondToEnableButton     = 5
	longTimeInSecondToEnableButton = 60
	firmwareInstallationTime       = 240 * time.Second
)

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
	if _, err := uiHelper.Client.CloseShimlessRMA(cleanupCtx, &empty.Empty{}); err != nil {
		testing.ContextLog(cleanupCtx, "Fail to close Shimless RMA client: ", err)
	}
	if err := uiHelper.RPCClient.Close(cleanupCtx); err != nil {
		testing.ContextLog(cleanupCtx, "Fail to close RPC client: ", err)
	}
}

// WelcomePageOperation handles all operations on Welcome Page.
func (uiHelper *UIHelper) WelcomePageOperation(ctx context.Context) error {
	return action.Combine("Welcome page operation",
		uiHelper.waitForPageToLoad("Chromebook repair", timeInSecondToLoadPage),
		uiHelper.waitAndClickButton("Get started", longTimeInSecondToEnableButton),
	)(ctx)
}

// ComponentsPageOperation handles all operations on Components Selection Page.
func (uiHelper *UIHelper) ComponentsPageOperation(ctx context.Context) error {
	return action.Combine("Components page operation",
		uiHelper.waitForPageToLoad("Select which components were replaced", timeInSecondToLoadPage),
		uiHelper.clickButton("Base Accelerometer"),
		uiHelper.clickButton("Next"),
	)(ctx)
}

// OwnerPageOperation handles all operations on Owner Selection Page.
func (uiHelper *UIHelper) OwnerPageOperation(destination DestinationOption) action.Action {
	return func(ctx context.Context) error {
		var buttonLabel string
		if destination == SameUser {
			buttonLabel = "Device will go to the same user"
		} else if destination == DifferentUser {
			buttonLabel = "Device will go to a different user or organization"
		} else {
			return errors.Errorf("%s is invalid destination", destination)
		}

		return action.Combine("Owner page operation",
			uiHelper.waitForPageToLoad("After repair, who will be using the device?", timeInSecondToLoadPage),
			uiHelper.clickRadioButton(buttonLabel),
			uiHelper.waitAndClickButton("Next", timeInSecondToEnableButton),
		)(ctx)
	}
}

// WriteProtectPageChooseRSU handles all operations on WP Page and select RSU.
func (uiHelper *UIHelper) WriteProtectPageChooseRSU(ctx context.Context) error {
	return uiHelper.writeProtectPageOperation("Perform RMA Server Unlock (RSU)")(ctx)
}

// WriteProtectPageChooseManual handles all operations on WP Page and select manual option.
func (uiHelper *UIHelper) WriteProtectPageChooseManual(ctx context.Context) error {
	return action.Combine("Write Protect page operation and choose manual",
		uiHelper.writeProtectPageOperation("Manually turn off"),
		uiHelper.disconnectBatteryByCr50(),
		uiHelper.changeWriteProtectStatus(servo.FWWPStateOff),
	)(ctx)

}

// WipeDevicePageOperation handles all operations on wipe device Page.
func (uiHelper *UIHelper) WipeDevicePageOperation(ctx context.Context) error {
	return action.Combine("Wipe Device page operation",
		uiHelper.waitForPageToLoad("Device is going to the same user. Erase user data?", timeInSecondToLoadPage),
		uiHelper.clickRadioButton("Erase all data"),
		uiHelper.waitAndClickButton("Next", timeInSecondToEnableButton),
	)(ctx)
}

// WriteProtectDisabledPageOperation handles all operations on Write Protect Disabled Page.
func (uiHelper *UIHelper) WriteProtectDisabledPageOperation(ctx context.Context) error {
	return action.Combine("Write Protect Disabled page operation",
		uiHelper.waitForPageToLoad("Write Protect is turned off", timeInSecondToLoadPage),
		uiHelper.clickButton("Next"),
	)(ctx)
}

// WriteProtectEnabledPageOperation handles all operations on Write Protect Enable Page.
func (uiHelper *UIHelper) WriteProtectEnabledPageOperation(ctx context.Context) error {
	return action.Combine("Write Protect Enabled page operation",
		uiHelper.waitForPageToLoad("Manually enable write-protect", timeInSecondToLoadPage),
		uiHelper.clickButton("Next"),
	)(ctx)
}

// FirmwareInstallationPageOperation handles all operations on Firmware Installation Page.
func (uiHelper *UIHelper) FirmwareInstallationPageOperation(ctx context.Context) error {
	if err := uiHelper.FirmwareHelper.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		return err
	}

	return action.Combine("Firmware Installation page operation",
		uiHelper.waitForPageToLoad("Install complete", timeInSecondToLoadPage),
	)(ctx)
}

// DeviceInformationPageOperation handles all operations on device information Page.
func (uiHelper *UIHelper) DeviceInformationPageOperation(ctx context.Context) error {
	return action.Combine("Device Information page operation",
		uiHelper.waitForPageToLoad("Please confirm device information", timeInSecondToLoadPage),
		uiHelper.clickButton("Next"),
	)(ctx)
}

// DeviceProvisionPageOperation handles all operations on device provisioning Page.
func (uiHelper *UIHelper) DeviceProvisionPageOperation(ctx context.Context) error {
	return action.Combine("Device Provision page operation",
		uiHelper.waitForPageToLoad("Provisioning the device…", timeInSecondToLoadPage),
		uiHelper.connectBatteryByCr50(),
		uiHelper.changeWriteProtectStatus(servo.FWWPStateOn),
	)(ctx)
}

// CalibratePageOperation handles all operations on calibrate Page.
func (uiHelper *UIHelper) CalibratePageOperation(ctx context.Context) error {
	return action.Combine("Calibrate page operation",
		uiHelper.waitForPageToLoad("Prepare to calibrate device components", timeInSecondToLoadPage),
		uiHelper.waitAndClickButton("Next", longTimeInSecondToEnableButton),
	)(ctx)
}

// FinalizingRepairPageOperation handles all operations on finalizing repair Page.
func (uiHelper *UIHelper) FinalizingRepairPageOperation(ctx context.Context) error {
	return action.Combine("Finalizing Repair page operation",
		uiHelper.waitForPageToLoad("Finalizing repair", timeInSecondToLoadPage),
	)(ctx)
}

// RepairCompletedPageOperation handles all operations on repair completed Page.
func (uiHelper *UIHelper) RepairCompletedPageOperation(ctx context.Context) error {
	return action.Combine("Repair Completed page operation",
		uiHelper.waitForPageToLoad("Almost done!", longTimeInSecondToEnableButton),
		uiHelper.clickButton("Reboot"),
	)(ctx)
}

// RSUPageOperation handles all operations on RSU Page.
func (uiHelper *UIHelper) RSUPageOperation(ctx context.Context) error {
	// Change battery status and WP status
	if err := action.Combine("Disconnect Battery and disable WP",
		uiHelper.disconnectBatteryByCr50(),
		uiHelper.changeWriteProtectStatus(servo.FWWPStateOff),
	)(ctx); err != nil {
		return err
	}

	if err := action.Combine("Click Challenge Code URL",
		uiHelper.waitForPageToLoad("Perform RMA Server Unlock", timeInSecondToLoadPage),
		uiHelper.clickLink("this URL"),
	)(ctx); err != nil {
		return err
	}
	url, err := uiHelper.retrieveTextByPrefix(ctx, "https")
	if err != nil {
		return err
	}
	// Extract the parameter from url.
	challengeCode, err := uiHelper.parseChallengeCode(url)
	if err != nil {
		return err
	}

	output, err := uiHelper.Dut.Conn().CommandContext(ctx, "/usr/local/bin/rma_reset", "-c", challengeCode).Output()
	if err != nil {
		return err
	}
	authCode, err := uiHelper.parseAuthCode(string(output))
	if err != nil {
		return err
	}

	return action.Combine("Enter unlock code and click Next",
		uiHelper.clickButton("Done"),
		uiHelper.enterIntoTextInput(authCode, "Enter the 8-character unlock code"),
		uiHelper.clickButton("Next"),
	)(ctx)

}

// BypassFirmwareInstallation will skip firmware installation.
func (uiHelper *UIHelper) BypassFirmwareInstallation(ctx context.Context) error {
	// This sleep is important since we need to wait for RMAD to update state file completed.
	testing.Sleep(ctx, 3*time.Second)
	if _, err := uiHelper.Client.BypassFirmwareInstallation(ctx, &empty.Empty{}); err != nil {
		return err
	}

	return uiHelper.Dut.Reboot(ctx)
}

// WaitForFirmwareInstallation will trigger and wait for firmware installation.
func (uiHelper *UIHelper) WaitForFirmwareInstallation(ctx context.Context) error {
	if err := uiHelper.FirmwareHelper.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Sleeping %s to wait for firmware installation", firmwareInstallationTime)
	return testing.Sleep(ctx, firmwareInstallationTime)
}

// SetupInitStatus setup initial status for shimless testing.
func (uiHelper *UIHelper) SetupInitStatus(ctx context.Context, enroll bool) error {
	// If error is raised, then Factory is already disabled.
	// Therefore, ignore any error.
	uiHelper.changeFactoryMode("disable")(ctx)

	return action.Combine("Setup init status for test",
		// Open CCD needs to be executed after disable Factory mode.
		// It is because disable Factory mode will also lock CCD.
		uiHelper.openCCDIfNotOpen(),
		uiHelper.changeWriteProtectStatus(servo.FWWPStateOn),
		uiHelper.changeEnrollment(enroll),
	)(ctx)
}

func (uiHelper *UIHelper) changeEnrollment(toEnroll bool) action.Action {
	return func(ctx context.Context) error {
		if _, err := uiHelper.Dut.Conn().CommandContext(ctx, "tpm_manager_client", "take_ownership").Output(); err != nil {
			return err
		}

		var flags string
		if toEnroll {
			flags = "--flags=0x40"
		} else {
			flags = "--flags=0"
		}

		_, err := uiHelper.Dut.Conn().CommandContext(ctx, "cryptohome", "--action=set_firmware_management_parameters", flags).Output()
		return err
	}
}

func (uiHelper *UIHelper) openCCDIfNotOpen() action.Action {
	return func(ctx context.Context) error {
		if val, err := uiHelper.FirmwareHelper.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
			return err
		} else if val != servo.Open {
			if err := uiHelper.FirmwareHelper.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
				return err
			}
		}
		return nil
	}
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

func (uiHelper *UIHelper) changeWriteProtectStatus(status servo.FWWPStateValue) action.Action {
	return func(ctx context.Context) error {
		return uiHelper.FirmwareHelper.Servo.SetFWWPState(ctx, status)
	}
}

func (uiHelper *UIHelper) changeFactoryMode(status string) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Dut.Conn().CommandContext(ctx, "gsctool", "-aF", status).Output()
		return err
	}
}

func (uiHelper *UIHelper) writeProtectPageOperation(radioButtonLabel string) action.Action {
	return action.Combine("Write Protect page operation",
		uiHelper.waitForPageToLoad("Select how you would like to turn off Write Protect", timeInSecondToLoadPage),
		uiHelper.clickRadioButton(radioButtonLabel),
		uiHelper.waitAndClickButton("Next", timeInSecondToEnableButton),
	)
}

func (uiHelper *UIHelper) waitAndClickButton(label string, timeInSecond int32) action.Action {
	return action.Combine("Wait and click button",
		func(ctx context.Context) error {
			_, err := uiHelper.Client.WaitUntilButtonEnabled(ctx, &pb.WaitUntilButtonEnabledRequest{
				Label:            label,
				DurationInSecond: timeInSecond,
			})
			return err
		},
		uiHelper.clickButton(label),
	)
}

func (uiHelper *UIHelper) clickButton(label string) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Client.LeftClickButton(ctx, &pb.LeftClickButtonRequest{
			Label: label,
		})
		return err
	}
}

func (uiHelper *UIHelper) waitForPageToLoad(title string, timeInSecond int32) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Client.WaitForPageToLoad(ctx, &pb.WaitForPageToLoadRequest{
			Title:            title,
			DurationInSecond: timeInSecond,
		})
		return err
	}
}

func (uiHelper *UIHelper) clickRadioButton(label string) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Client.LeftClickRadioButton(ctx, &pb.LeftClickRadioButtonRequest{
			Label: label,
		})
		return err
	}
}

func (uiHelper *UIHelper) clickLink(label string) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Client.LeftClickLink(ctx, &pb.LeftClickLinkRequest{
			Label: label,
		})
		return err
	}
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

func (uiHelper *UIHelper) parseChallengeCode(url string) (string, error) {
	re := regexp.MustCompile("challenge=([^&]*)")
	match := re.FindStringSubmatch(url)
	if match == nil {
		return "", errors.New("fail to get Challenge Code")
	}
	return match[1], nil
}

func (uiHelper *UIHelper) parseAuthCode(raw string) (string, error) {
	re := regexp.MustCompile(`Authcode:\s+([a-zA-Z0-9]*)`)
	match := re.FindStringSubmatch(raw)
	if match == nil {
		return "", errors.New("fail to get Auth Code")
	}

	return match[1], nil
}

func (uiHelper *UIHelper) enterIntoTextInput(content, textInputName string) action.Action {
	return func(ctx context.Context) error {
		_, err := uiHelper.Client.EnterIntoTextInput(ctx, &pb.EnterIntoTextInputRequest{
			TextInputName: textInputName,
			Content:       content,
		})
		return err
	}
}

func (uiHelper *UIHelper) disconnectBatteryByCr50() action.Action {
	return func(ctx context.Context) error {
		return uiHelper.FirmwareHelper.Servo.RunCR50Command(ctx, "bpforce disconnect atboot")
	}
}

func (uiHelper *UIHelper) connectBatteryByCr50() action.Action {
	return func(ctx context.Context) error {
		return uiHelper.FirmwareHelper.Servo.RunCR50Command(ctx, "bpforce follow_batt_pres atboot")
	}
}
