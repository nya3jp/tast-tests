// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
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

const (
	waitForInstallAttributesTimeout = 30 * time.Second
	testAttributesUndefined         = "Naproxen"
	tamperedAttributes              = "Methadone"
	databasePath                    = "/home/.shadow/install_attributes.pb"
)

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
func getInstallAttributesStates(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) (isReady, isInitialized, isInvalid, isFirstInstall, isSecure bool, count int, returnError error) {
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
		return
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

// waitForInstallAttributes waits for install attributes to be ready.
func waitForInstallAttributes(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
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
	}, &testing.PollOptions{Timeout: waitForInstallAttributesTimeout}); err != nil {
		return errors.Wrap(err, "failed waiting for install attributes after taking ownership")
	}
	return nil
}

// takeOwnershipAndWaitForInstallAttributes takes ownership and wait for install attributes to be ready.
func takeOwnershipAndWaitForInstallAttributes(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, helper *hwsecremote.HelperRemote) error {
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		return errors.Wrap(err, "time out waiting for TPM to be ready")
	}

	return waitForInstallAttributes(ctx, utility)
}

// checkAllTestAttributes is a helper function that checks the install attributes retrieved through cryptohome's API is what we are expecting.
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

// attemptChangeAndCheckShouldSucceed checks that install attributes are settable when it should be, it also verifies the install attributes values.
func attemptChangeAndCheckShouldSucceed(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	if err := utility.InstallAttributesSet(ctx, testAttributes[0], testValues[1]); err != nil {
		return errors.Wrap(err, "failed to set install attributes when it should still be settable")
	}

	if err := utility.InstallAttributesSet(ctx, testAttributes[0], testValues[0]); err != nil {
		return errors.Wrap(err, "failed to set install attributes back to its original value when it should still be settable")
	}

	// Lastly, check the attributes.
	return checkAllTestAttributes(ctx, utility)
}

// attemptChangeAndCheckShouldFail checks that install attributes are not settable, it also verifies the install attributes values.
func attemptChangeAndCheckShouldFail(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	if err := utility.InstallAttributesSet(ctx, testAttributes[0], testValues[1]); err == nil {
		return errors.New("setting install attributes to a different value succeeded when it shouldn't")
	}

	if err := utility.InstallAttributesSet(ctx, testAttributes[0], testValues[0]); err == nil {
		return errors.New("setting install attributes to same value succeeded when it shouldn't")
	}

	if err := utility.InstallAttributesSet(ctx, testAttributesUndefined, testValues[0]); err == nil {
		return errors.New("setting previously undefined install attributes succeeded when it shouldn't")
	}

	// Lastly, check the attributes.
	return checkAllTestAttributes(ctx, utility)
}

// tamperWithInstallAttributes attempts to modify the install attributes by directly modifying its database.
func tamperWithInstallAttributes(ctx context.Context, r hwsec.CmdRunner) error {
	// Check that the tampered string is the same length as the original attribute.
	if len(tamperedAttributes) != len(testAttributes[0]) {
		panic("Incorrect tampered attribute string length.")
	}

	// Check that the string is in the file.
	if _, err := r.Run(ctx, "grep", "-q", testAttributes[0], databasePath); err != nil {
		// The attribute is not found in the database.
		return errors.Wrapf(err, "the database doesn't contain the test attributes %q", testAttributes[0])
	}

	// Now replace the string, thus tampering the database.
	if _, err := r.Run(ctx, "sed", "-bi", fmt.Sprintf("s/%s/%s/g", testAttributes[0], tamperedAttributes), databasePath); err != nil {
		// Failed to replace the attribute.
		return errors.Wrap(err, "failed to replace the attribute")
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
	}

	// Take ownership then wait for install attributes.
	if err := takeOwnershipAndWaitForInstallAttributes(ctx, utility, helper); err != nil {
		s.Fatal("Failed to take ownership or wait for install attributes: ", err)
	}

	// Now the install attributes are ready, let's write some attributes and see if it works.
	for i, attributeName := range testAttributes {
		attributeValue := testValues[i]
		if err = utility.InstallAttributesSet(ctx, attributeName, attributeValue); err != nil {
			s.Fatal("Failed to set install attributes "+attributeName+" to value "+attributeValue+": ", err)
		}
	}

	// Check attributes and test setting after finalizing.
	if err = attemptChangeAndCheckShouldSucceed(ctx, utility); err != nil {
		s.Fatal("Check install attributes failed pre finalization: ", err)
	}

	// Next finalize it.
	if err = utility.InstallAttributesFinalize(ctx); err != nil {
		s.Fatal("Failed to finalize install attributes: ", err)
	}

	// Check install attributes right after finalizing.
	isReady, isInitialized, isInvalid, isFirstInstall, _, count, err = getInstallAttributesStates(ctx, utility)
	if err != nil {
		s.Fatal("Failed to parse cryptohoattemptChangeAndCheckShouldFailme status: ", err)
	}

	if !isReady || !isInitialized || isInvalid || isFirstInstall || count != 3 {
		s.Fatalf("Unexpected Install Attributes state after install attribute finalize; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
	}

	// Check that trying to set install attribute now fails.
	if err := attemptChangeAndCheckShouldFail(ctx, utility); err != nil {
		s.Fatal("Checking install attributes failed post finalization: ", err)
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
	}

	// Recheck the install attributes
	if err := attemptChangeAndCheckShouldFail(ctx, utility); err != nil {
		s.Fatal("Checking install attributes failed post finalization reboot: ", err)
	}

	// Now tamper with the attributes.
	if err := tamperWithInstallAttributes(ctx, r); err != nil {
		s.Fatal("Failed to tamper with the install attributes database: ", err)
	}

	// Reboot so that it'll take effect.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	// Check install attributes after tampering with install attributes.
	isReady, isInitialized, isInvalid, isFirstInstall, _, count, err = getInstallAttributesStates(ctx, utility)
	if err != nil {
		s.Fatal("Failed to parse cryptohome status: ", err)
	}
	if !isReady || !isInitialized || isInvalid || !isFirstInstall || count != 0 {
		s.Fatalf("Unexpected Install Attributes state after tampering with install attributes; ready=%t, initialized=%t, invalid=%t, firstInstall=%t, count=%d", isReady, isInitialized, isInvalid, isFirstInstall, count)
	}

	// Check that neither the original nor the tampered attributes are readable.
	if readbackValue, err := utility.InstallAttributesGet(ctx, testAttributes[0]); err == nil {
		s.Fatalf("Able to read install attributes %q after tampering the database, got %q", testAttributes[0], readbackValue)
	}
	if readbackValue, err := utility.InstallAttributesGet(ctx, tamperedAttributes); err == nil {
		s.Fatalf("Able to read install attributes %q after tampering the database, got %q", tamperedAttributes, readbackValue)
	}
}
