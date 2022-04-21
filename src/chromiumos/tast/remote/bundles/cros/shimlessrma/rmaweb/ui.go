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
	// Client contains Shimless RMA App client.
	Client pb.AppServiceClient
}

// NewUIHelper creates UIHelper.
func NewUIHelper(client pb.AppServiceClient) *UIHelper {
	uiHelper := &UIHelper{client}
	return uiHelper
}

// WelcomePageOperation handles all operations on Welcome Page.
func (uiHelper *UIHelper) WelcomePageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Chromebook repair", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Get started >", longTimeInSecondToEnableButton); err != nil {
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
	if err := uiHelper.clickButton(ctx, "Next >"); err != nil {
		return err
	}

	return nil
}

// OwnerPageOperation handles all operations on Owner Selection Page.
func (uiHelper *UIHelper) OwnerPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "After repair, who will be using the device?", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, "Device will go to the same owner"); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectPageOperation handles all operations on WP Page.
func (uiHelper *UIHelper) WriteProtectPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Select how you would like to disable write-protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, "Manually disable write-protect"); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WipeDevicePageOperation handles all operations on wipe device Page.
func (uiHelper *UIHelper) WipeDevicePageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Preserve Device Data", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickRadioButton(ctx, "Wipe device"); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next >", timeinSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// WriteProtectDisabledPageOperation handles all operations on Write Protect Disabled Page.
func (uiHelper *UIHelper) WriteProtectDisabledPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Write Protect is turned off", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next >"); err != nil {
		return err
	}

	return nil
}

// WriteProtectEnabledPageOperation handles all operations on Write Protect Enable Page.
func (uiHelper *UIHelper) WriteProtectEnabledPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Manually enable write-protect", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next >"); err != nil {
		return err
	}

	return nil
}

// FirmwareInstallationPageOperation handles all operations on Firmware Installation Page.
func (uiHelper *UIHelper) FirmwareInstallationPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Install firmware image", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// DeviceInformationPageOperation handles all operations on device information Page.
func (uiHelper *UIHelper) DeviceInformationPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Please confirm device information", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Next >"); err != nil {
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
	if err := uiHelper.waitAndClickButton(ctx, "Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// FinalizingRepairPageOperation handles all operations on finalizing repair Page.
func (uiHelper *UIHelper) FinalizingRepairPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Finalizing repair", timeInSecondToLoadPage); err != nil {
		return err
	}
	if err := uiHelper.waitAndClickButton(ctx, "Next >", longTimeInSecondToEnableButton); err != nil {
		return err
	}

	return nil
}

// RepairCompeletedPageOperation handles all operations on repair completed Page.
func (uiHelper *UIHelper) RepairCompeletedPageOperation(ctx context.Context) error {
	if err := uiHelper.waitForPageToLoad(ctx, "Repair completed", longTimeInSecondToEnableButton); err != nil {
		return err
	}
	if err := uiHelper.clickButton(ctx, "Reboot"); err != nil {
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
