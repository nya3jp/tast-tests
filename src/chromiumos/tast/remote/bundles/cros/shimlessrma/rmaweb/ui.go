// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rmaweb contains web-related common functions used in the Shimless RMA app.
package rmaweb

import (
	"context"

	pb "chromiumos/tast/services/cros/shimlessrma"
)

const timeInSecondToLoadPage = 30
const timeinSecondToEnableButton = 5
const longTimeInSecondToEnableButton = 60

// UIHelper holds the resources required to communicate with Shimless RMA App.
type UIHelper struct {
	// Ctx contains context used to communicate with Shimless RMA App.
	Ctx context.Context
	// Client contains Shimless RMA App client.
	Client pb.AppServiceClient
}

// NewUIHelper creates UIHelper.
func NewUIHelper(ctx context.Context, client pb.AppServiceClient) *UIHelper {
	ui := &UIHelper{ctx, client}
	return ui
}

// WelcomePageOperation handles all operations on Welcome Page.
func (ui *UIHelper) WelcomePageOperation() error {
	if err := ui.waitForPageToLoad("Chromebook repair", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Get started >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// ComponentsPageOperation handles all operations on Components Selection Page.
func (ui *UIHelper) ComponentsPageOperation() error {
	if err := ui.waitForPageToLoad("Select which components were replaced", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickButton("Base Accelerometer"); err != nil {
		return err
	}
	if err := ui.clickButton("Next >"); err != nil {
		return err
	}

	return nil
}

// OwnerPageOperation handles all operations on Owner Selection Page.
func (ui *UIHelper) OwnerPageOperation() error {
	if err := ui.waitForPageToLoad("After repair, who will be using the device?", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickRadioButton("Device will go to the same owner"); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectPageOperation handles all operations on WP Page.
func (ui *UIHelper) WriteProtectPageOperation() error {
	if err := ui.waitForPageToLoad("Select how you would like to disable write-protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickRadioButton("Manually disable write-protect"); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WipeDevicePageOperation handles all operations on wipe device Page.
func (ui *UIHelper) WipeDevicePageOperation() error {
	if err := ui.waitForPageToLoad("Preserve Device Data", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickRadioButton("Wipe device"); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectDisabledPageOperation handles all operations on Write Protect Disabled Page.
func (ui *UIHelper) WriteProtectDisabledPageOperation() error {
	if err := ui.waitForPageToLoad("Write Protect is turned off", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickButton("Next >"); err != nil {
		return err
	}

	return nil
}

// WriteProtectEnabledPageOperation handles all operations on Write Protect Enable Page.
func (ui *UIHelper) WriteProtectEnabledPageOperation() error {
	if err := ui.waitForPageToLoad("Manually enable write-protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickButton("Next >"); err != nil {
		return err
	}

	return nil
}

// FirmwareInstallationPageOperation handles all operations on Firmware Installation Page.
func (ui *UIHelper) FirmwareInstallationPageOperation() error {
	if err := ui.waitForPageToLoad("Install firmware image", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// DeviceInformationPageOperation handles all operations on device information Page.
func (ui *UIHelper) DeviceInformationPageOperation() error {
	if err := ui.waitForPageToLoad("Please confirm device information", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.clickButton("Next >"); err != nil {
		return err
	}

	return nil
}

// DeviceProvisionPageOperation handles all operations on device provisioning Page.
func (ui *UIHelper) DeviceProvisionPageOperation() error {
	if err := ui.waitForPageToLoad("Provisioning the device…", timeInSecondToLoadPage); err != nil {
		return err
	}

	return nil
}

// CalibratePageOperation handles all operations on calibrate Page.
func (ui *UIHelper) CalibratePageOperation() error {
	if err := ui.waitForPageToLoad("Prepare to calibrate device components", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// FinalizingRepairPageOperation handles all operations on finalizing repair Page.
func (ui *UIHelper) FinalizingRepairPageOperation() error {
	if err := ui.waitForPageToLoad("Finalizing repair", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := ui.waitAndClickButton("Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// RepairCompeletedPageOperation handles all operations on repair completed Page.
func (ui *UIHelper) RepairCompeletedPageOperation() error {
	if err := ui.waitForPageToLoad("Repair completed", longTimeInSecondToEnableButton); err != nil {
		return err
	}
	if err := ui.clickButton("Reboot"); err != nil {
		return err
	}

	return nil
}

func (ui *UIHelper) waitAndClickButton(label string, timeInSecond int32) error {
	if _, err := ui.Client.WaitUntilButtonEnabled(ui.Ctx, &pb.WaitUntilButtonEnabledRequest{
		Label:            label,
		DurationInSecond: timeInSecond,
	}); err != nil {
		return err
	}

	return ui.clickButton(label)
}

func (ui *UIHelper) clickButton(label string) error {
	_, err := ui.Client.LeftClickButton(ui.Ctx, &pb.LeftClickButtonRequest{
		Label: label,
	})
	return err
}

func (ui *UIHelper) waitForPageToLoad(title string, timeInSecond int32) error {
	_, err := ui.Client.WaitForPageToLoad(ui.Ctx, &pb.WaitForPageToLoadRequest{
		Title:            title,
		DurationInSecond: timeInSecond,
	})
	return err
}

func (ui *UIHelper) clickRadioButton(label string) error {
	_, err := ui.Client.LeftClickRadioButton(ui.Ctx, &pb.LeftClickRadioButtonRequest{
		Label: label,
	})

	return err
}
