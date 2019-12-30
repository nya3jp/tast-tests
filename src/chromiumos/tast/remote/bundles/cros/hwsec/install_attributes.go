// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: InstallAttributes,
		Desc: "Checks that install attributes works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

var testAttributes = [...]string{"Ibuprofen", "Acetaminophen", "Acetylsalicylic Acid"}
var testValues = [...]string{"C13H18O2", "C8H9NO2", "C9H8O4"}

// parseStatusStringForInstallAttributesState returns initialized, invalid, firstInstall and any error encountered.
func parseStatusStringForInstallAttributesState(obj map[string]interface{}) (bool, bool, bool, error) {
	installattrs, ok := obj["installattrs"].(map[string]interface{})
	if !ok {
		return false, false, false, errors.New("no installattrs in cryptohome status")
	}

	initialized, ok := installattrs["initialized"].(bool)
	if !ok {
		return false, false, false, errors.New("installattrs.initialized doesn't exist or have incorrect type in cryptohome status")
	}

	invalid, ok := installattrs["invalid"].(bool)
	if !ok {
		return false, false, false, errors.New("installattrs.invalid doesn't exist or have incorrect type in cryptohome status")
	}

	firstInstall, ok := installattrs["first_install"].(bool)
	if !ok {
		return false, false, false, errors.New("installattrs.first_install doesn't exist or have incorrect type in cryptohome status")
	}

	return initialized, invalid, firstInstall, nil
}

// getInstallAttributesStates returns isReady, isInitialized, isInvalid, isFirstInstall, isSecure, count and any error encountered.
func getInstallAttributesStates(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) (isReady bool, isInitialized bool, isInvalid bool, isFirstInstall bool, isSecure bool, count int, returnError error) {
	// Default return values.
	isReady = false
	isInitialized = false
	isInvalid = false
	isFirstInstall = false
	count = -1

	// Get the values through GetStatusJSON().
	obj, err := utility.GetStatusJSON(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get cryptohome status")
	}

	isInitialized, isInvalidFromStatusString, isFirstInstallFromStatusString, err := parseStatusStringForInstallAttributesState(obj)
	if err != nil {
		returnError = errors.Wrap(err, "failed to parse install attributes json")
		return
	}

	// Get the values through individual dbus calls.
	count, err = utility.InstallAttributesCount(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get count")
		return
	}

	isReady, err = utility.InstallAttributesIsReady(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get is ready")
		return
	}

	isSecure, err = utility.InstallAttributesIsSecure(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get is secure")
		return
	}

	isInvalid, err = utility.InstallAttributesIsInvalid(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get is invalid")
		return
	}
	if isInvalid != isInvalidFromStatusString {
		returnError = errors.Errorf("mismatch between isInvalid from status string (%t) and dbus method %t", isInvalidFromStatusString, isInvalid)
		return
	}

	isFirstInstall, err = utility.InstallAttributesIsFirstInstall(ctx)
	if err != nil {
		returnError = errors.Wrap(err, "failed to get is first install")
		return
	}
	if isFirstInstall != isFirstInstallFromStatusString {
		returnError = errors.Errorf("mismatch between isFirstInstall from status string (%t) and dbus method (%t)", isFirstInstallFromStatusString, isFirstInstall)
		return
	}

	return
}

func checkAllTestAttributes(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	for i, attributeName := range testAttributes {
		attributeValue := testValues[i]
		readbackValue, err := utility.InstallAttributesGet(ctx, attributeName)
		if err != nil {
			return errors.Wrapf(err, "failed to get install attributes %q", attributeName)
		}
		if readbackValue != attributeValue {
			return errors.Errorf("incorrect attribute value for attribute %q, expected %q got %q", attributeName, attributeValue, readbackValue)
		}
	}
	return nil
}

func InstallAttributes(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := hwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	// Check install attributes right after resetting the TPM.
	isReady, isInitialized, isInvalid, isFirstInstall, _, count, err := getInstallAttributesStates(ctx, utility)
	if err != nil {
		s.Fatal("Failed to parse cryptohome status: ", err)
	}

	if isReady || isInitialized || isInvalid || isFirstInstall || count != 0 {
		s.Fatalf("Unexpected Install Attributes state after TPM reset; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
		return
	}

	if err = helper.EnsureTPMIsReady(ctx, 1000*100); err != nil {
		s.Error("Time out waiting for TPM to be ready: ", err)
		return
	}

	// Wait for, and check TPM attributes after taking ownership.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isReady, isInitialized, isInvalid, isFirstInstall, _, count, err := getInstallAttributesStates(ctx, utility)
		if err != nil {
			return err
		}

		if !isReady || !isInitialized || isInvalid || !isFirstInstall || count != 0 {
			return errors.Errorf("unexpected Install Attributes state after taking ownership; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed waiting for install attributes after taking ownership: ", err)
	}

	// Now the install attributes are ready, let's write some attributes and see if it works.
	for i, attributeName := range testAttributes {
		attributeValue := testValues[i]
		if err = utility.InstallAttributesSet(ctx, attributeName, attributeValue); err != nil {
			s.Fatal("Failed to set install attributes "+attributeName+" to value "+attributeValue+": ", err)
		}
	}

	// Read it back to check if it's OK.
	if err = checkAllTestAttributes(ctx, utility); err != nil {
		s.Fatal("Check all attributes failed: ", err)
	}

	// Next finalize it.
	if err = utility.InstallAttributesFinalize(ctx); err != nil {
		s.Fatal("Failed to finalize install attributes: ", err)
	}

	// Check install attributes right after finalizing.
	isReady, isInitialized, isInvalid, isFirstInstall, _, count, err = getInstallAttributesStates(ctx, utility)
	if err != nil {
		s.Fatal("Failed to parse cryptohome status: ", err)
	}

	if !isReady || !isInitialized || isInvalid || isFirstInstall || count != 3 {
		s.Fatalf("Unexpected Install Attributes state after install attribute finalize; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
		return
	}

	// Check that trying to set install attribute now fails.
	err = utility.InstallAttributesSet(ctx, testAttributes[0], testValues[1])
	if err == nil {
		s.Fatal("Set Attribute is successful post finalization")
	}

	// Recheck the install attributes
	if err = checkAllTestAttributes(ctx, utility); err != nil {
		s.Fatal("Check all attributes failed: ", err)
	}

	// Reboot to check that everything is alright after reboot.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	// Check install attributes after reboot.
	isReady, isInitialized, isInvalid, isFirstInstall, _, count, err = getInstallAttributesStates(ctx, utility)
	if err != nil {
		s.Fatal("Failed to parse cryptohome status: ", err)
	}

	if !isReady || !isInitialized || isInvalid || isFirstInstall || count != 3 {
		s.Fatalf("Unexpected Install Attributes state after reboot; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
		return
	}

	// Recheck the install attributes
	if err = checkAllTestAttributes(ctx, utility); err != nil {
		s.Fatal("Check all attributes failed: ", err)
	}
}
