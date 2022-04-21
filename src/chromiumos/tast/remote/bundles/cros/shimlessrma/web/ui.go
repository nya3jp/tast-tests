// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package web contains web-related common functions used in the Shimless RMA app.
package web

import (
	"context"

	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
)

const timeInSecondToLoadPage = 30
const timeinSecondToEnableButton = 5
const longTimeInSecondToEnableButton = 60

// WelcomePageOperation handles all operations on Welcome Page.
func WelcomePageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Chromebook repair", timeInSecondToLoadPage)
	waitAndClickButton(ctx, s, client, "Get started >", longTimeInSecondToEnableButton)
}

// ComponentsPageOperation handles all operations on Components Selection Page.
func ComponentsPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Select which components were replaced", timeInSecondToLoadPage)
	clickButton(ctx, s, client, "Base Accelerometer")
	clickButton(ctx, s, client, "Next >")
}

// OwnerPageOperation handles all operations on Owner Selection Page.
func OwnerPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "After repair, who will be using the device?", timeInSecondToLoadPage)
	clickRadioButton(ctx, s, client, "Device will go to the same owner")
	waitAndClickButton(ctx, s, client, "Next >", timeinSecondToEnableButton)
}

// WriteProtectPageOperation handles all operations on WP Page.
func WriteProtectPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Select how you would like to disable write-protect", timeInSecondToLoadPage)
	clickRadioButton(ctx, s, client, "Manually disable write-protect")
	waitAndClickButton(ctx, s, client, "Next >", timeinSecondToEnableButton)
}

// WipeDevicePageOperation handles all operations on wipe device Page.
func WipeDevicePageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Preserve Device Data", timeInSecondToLoadPage)
	clickRadioButton(ctx, s, client, "Wipe device")
	waitAndClickButton(ctx, s, client, "Next >", timeinSecondToEnableButton)
}

// WriteProtectDisabledPageOperation handles all operations on Write Protect Disabled Page.
func WriteProtectDisabledPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Write Protect is turned off", timeInSecondToLoadPage)
	clickButton(ctx, s, client, "Next >")
}

// WriteProtectEnabledPageOperation handles all operations on Write Protect Enable Page.
func WriteProtectEnabledPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Manually enable write-protect", timeInSecondToLoadPage)
	clickButton(ctx, s, client, "Next >")
}

// FirmwareInstallationPageOperation handles all operations on Firmware Installation Page.
func FirmwareInstallationPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Install firmware image", timeInSecondToLoadPage)
	waitAndClickButton(ctx, s, client, "Next >", longTimeInSecondToEnableButton)
}

// DeviceInformationPageOperation handles all operations on device information Page.
func DeviceInformationPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Please confirm device information", timeInSecondToLoadPage)
	clickButton(ctx, s, client, "Next >")
}

// DeviceProvisionPageOperation handles all operations on device provisioning Page.
func DeviceProvisionPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Provisioning the device…", timeInSecondToLoadPage)
	waitAndClickButton(ctx, s, client, "Next >", longTimeInSecondToEnableButton)
}

// CalibratePageOperation handles all operations on calibrate Page.
func CalibratePageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Prepare to calibrate device components", timeInSecondToLoadPage)
	waitAndClickButton(ctx, s, client, "Next >", longTimeInSecondToEnableButton)
}

// FinalizingRepairPageOperation handles all operations on finalizing repair Page.
func FinalizingRepairPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Finalizing repair", timeInSecondToLoadPage)
	waitAndClickButton(ctx, s, client, "Next >", longTimeInSecondToEnableButton)
}

// RepairCompeletedPageOperation handles all operations on repair completed Page.
func RepairCompeletedPageOperation(ctx context.Context, s *testing.State, client pb.AppServiceClient) {
	waitForPageToLoad(ctx, s, client, "Repair completed", timeInSecondToLoadPage)
	clickButton(ctx, s, client, "Reboot")
}

func waitAndClickButton(ctx context.Context, s *testing.State, client pb.AppServiceClient, label string, timeInSecond int32) {
	if _, err := client.WaitUntilButtonEnabled(ctx, &pb.WaitUntilButtonEnabledRequest{
		Label:            label,
		DurationInSecond: timeInSecond,
	}); err != nil {
		s.Fatalf("%s button was not enabled: %v", label, err)
	}

	clickButton(ctx, s, client, label)
}

func clickButton(ctx context.Context, s *testing.State, client pb.AppServiceClient, label string) {
	if _, err := client.LeftClickButton(ctx, &pb.LeftClickButtonRequest{
		Label: label,
	}); err != nil {
		s.Fatalf("Failed to click %s button: %v", label, err)
	}
}

func waitForPageToLoad(ctx context.Context, s *testing.State, client pb.AppServiceClient, title string, timeInSecond int32) {
	if _, err := client.WaitForPageToLoad(ctx, &pb.WaitForPageToLoadRequest{
		Title:            title,
		DurationInSecond: timeInSecond,
	}); err != nil {
		s.Fatal("Failed to load Firmware Installation page: ", err)
	}
}

func clickRadioButton(ctx context.Context, s *testing.State, client pb.AppServiceClient, label string) {
	if _, err := client.LeftClickRadioButton(ctx, &pb.LeftClickRadioButtonRequest{
		Label: label,
	}); err != nil {
		s.Fatalf("Failed to select %s option: %v", label, err)
	}
}
