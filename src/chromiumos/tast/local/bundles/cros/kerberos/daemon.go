// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"

	kp "chromiumos/system_api/kerberos_proto"
	"chromiumos/tast/local/bundles/cros/kerberos/client"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Daemon,
		Desc: "Verifies that the Kerberos system dameon works as expected",
		Contacts: []string{
			"ljusten@chromium.org",
			"rsorokin@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func Daemon(ctx context.Context, s *testing.State) {
	const (
		user          = "user@EXAMPLE.COM"
		password      = "fakepw123"
		validConfig   = "[libdefaults]\nforwardable=false"
		invalidConfig = "[libdefaults]\nallow_weak_crypto=true"
	)

	k, err := client.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Kerberos binding: ", err)
	}

	// Wipe any existing accounts from previous tests.
	clearResp, err := k.ClearAccounts(ctx)
	if err != nil {
		s.Fatal("ClearAccounts failed. D-Bus error: ", err)
	}
	if *clearResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("ClearAccounts failed unexpectedly with error %q", clearResp.Error.String())
	}

	// Add an account.
	addResp, err := k.AddAccount(ctx, user)
	if err != nil {
		s.Fatal("AddAccount failed. D-Bus error: ", err)
	}
	if *addResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("AddAccount failed unexpectedly with error %q", addResp.Error.String())
	}

	// Set a valid config on the account.
	setResp, err := k.SetConfig(ctx, user, validConfig)
	if err != nil {
		s.Fatal("SetConfig failed. D-Bus error: ", err)
	}
	if *setResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("SetConfig failed unexpectedly with error %q", setResp.Error.String())
	}

	// Set an invalid config on the account.
	setResp, err = k.SetConfig(ctx, user, invalidConfig)
	if err != nil {
		s.Fatal("SetConfig failed. D-Bus error: ", err)
	}
	if *setResp.Error != kp.ErrorType_ERROR_BAD_CONFIG {
		s.Fatalf("SetConfig returned unexpected error: got %q; want \"ErrorType_ERROR_BAD_CONFIG\"", setResp.Error.String())
	}

	// Find out why the config was invalid.
	validateResp, err := k.ValidateConfig(ctx, invalidConfig)
	if err != nil {
		s.Fatal("ValidateConfig failed. D-Bus error: ", err)
	}
	if *validateResp.Error != kp.ErrorType_ERROR_BAD_CONFIG {
		s.Fatalf("ValidateConfig returned unexpected error: got %q; want \"ErrorType_ERROR_BAD_CONFIG\"", validateResp.Error.String())
	}
	if *validateResp.ErrorInfo.Code != kp.ConfigErrorCode_CONFIG_ERROR_KEY_NOT_SUPPORTED {
		s.Fatalf("ValidateConfig returned unexpected config error code: got %q; want \"ConfigErrorCode_CONFIG_ERROR_KEY_NOT_SUPPORTED\"", validateResp.ErrorInfo.Code)
	}
	if *validateResp.ErrorInfo.LineIndex != 1 {
		s.Fatalf("ValidateConfig returned unexpected line index: got %d; want 1", validateResp.ErrorInfo.LineIndex)
	}

	// Acquire a Kerberos ticket.
	acquireTgtResp, err := k.AcquireKerberosTgt(ctx, user, password /*rememberPassword=*/, true /*useLoginPassword=*/, false)
	if err != nil {
		s.Fatal("AcquireKerberosTgt failed. D-Bus error: ", err)
	}
	if *acquireTgtResp.Error != kp.ErrorType_ERROR_CONTACTING_KDC_FAILED {
		s.Fatalf("AcquireKerberosTgt returned unexpected error: got %q; want \"ErrorType_ERROR_CONTACTING_KDC_FAILED\"", acquireTgtResp.Error.String())
	}

	// List account and verify the data.
	listResp, err := k.ListAccounts(ctx)
	if err != nil {
		s.Fatal("ListAccounts failed. D-Bus error: ", err)
	}
	if *listResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("ListAccounts failed unexpectedly with error %q", listResp.Error.String())
	}
	if len(listResp.Accounts) != 1 {
		s.Fatalf("Unexpected accounts len: got %d; want 1", len(listResp.Accounts))
	}

	acc := listResp.Accounts[0]
	if *acc.PrincipalName != user {
		s.Fatalf("Unexpected account 'PrincipalName': got %q; expected %q", *acc.PrincipalName, user)
	}
	if *acc.IsManaged {
		s.Fatalf("Unexpected account 'IsManaged' flag: got %t; expected false", *acc.IsManaged)
	}
	if !*acc.PasswordWasRemembered {
		s.Fatalf("Unexpected account 'PasswordWasRemembered' flag: got %t; expected true", *acc.PasswordWasRemembered)
	}
	if *acc.UseLoginPassword {
		s.Fatalf("Unexpected account 'UseLoginPassword' flag: got %t; expected false", *acc.UseLoginPassword)
	}
	if string(acc.Krb5Conf) != validConfig {
		s.Fatalf("Unexpected account 'PrincipalName': got %q; expected %q", string(acc.Krb5Conf), validConfig)
	}
	if acc.TgtValiditySeconds != nil {
		s.Fatalf("Unexpected account 'TgtValiditySeconds': got %d; want nil", *acc.TgtValiditySeconds)
	}
	if acc.TgtRenewalSeconds != nil {
		s.Fatalf("Unexpected account 'GetTgtRenewalSeconds': got %d; want nil", *acc.TgtRenewalSeconds)
	}

	// Get files.
	getFilesResp, err := k.GetKerberosFiles(ctx, user)
	if err != nil {
		s.Fatal("GetKerberosFiles failed. D-Bus error: ", err)
	}
	if *getFilesResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("GetKerberosFiles failed unexpectedly with error %q", getFilesResp.Error.String())
	}
	if len(getFilesResp.Files.Krb5Conf) != 0 {
		s.Fatalf("Unexpected Krb5Conf length: got %d; expected 0", len(getFilesResp.Files.Krb5Conf))
	}
	if len(getFilesResp.Files.Krb5Cc) != 0 {
		s.Fatalf("Unexpected Krb5Cc length: got %d; expected 0", len(getFilesResp.Files.Krb5Cc))
	}

	// Remove account again.
	removeResp, err := k.RemoveAccount(ctx, user)
	if err != nil {
		s.Fatal("RemoveAccount failed. D-Bus error: ", err)
	}
	if *removeResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("RemoveAccount failed unexpectedly with error %q", removeResp.Error.String())
	}

	// Verify that account list is empty.
	listResp, err = k.ListAccounts(ctx)
	if err != nil {
		s.Fatal("ListAccounts failed. D-Bus error: ", err)
	}
	if *listResp.Error != kp.ErrorType_ERROR_NONE {
		s.Fatalf("ListAccounts failed unexpectedly with error %q", listResp.Error.String())
	}
	if len(listResp.Accounts) != 0 {
		s.Fatalf("Unexpected accounts len: got %d; want 0", len(listResp.Accounts))
	}
}
