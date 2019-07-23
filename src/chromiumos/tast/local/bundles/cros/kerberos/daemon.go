// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"

	kp "chromiumos/system_api/kerberos_proto"
	"chromiumos/tast/local/kerberos"
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
	assertErrorEquals := func(err error, actual *kp.ErrorType, expected kp.ErrorType, method string) {
		if err != nil {
			s.Fatal(method+" failed. D-Bus error ", err)
		}
		if *actual != expected {
			s.Fatalf("%s failed. Actual: %q, expected: %q", method, actual.String(), expected.String())
		}
	}

	assertOk := func(err error, actual *kp.ErrorType, method string) {
		assertErrorEquals(err, actual, kp.ErrorType_ERROR_NONE, method)
	}

	assertIntEquals := func(actual, expected int, errMsg string) {
		if actual != expected {
			s.Fatalf("%s mismatch. Actual: %d, expected: %d", errMsg, actual, expected)
		}
	}

	assertBoolEquals := func(actual, expected bool, errMsg string) {
		if actual != expected {
			s.Fatalf("%s mismatch. Actual: %t, expected: %t", errMsg, actual, expected)
		}
	}

	assertStringEquals := func(actual, expected, errMsg string) {
		if actual != expected {
			s.Fatalf("%s mismatch. Actual: %q, expected: %q", errMsg, actual, expected)
		}
	}

	assertConfigErrorEquals := func(actual, expected kp.ConfigErrorCode, method string) {
		if actual != expected {
			s.Fatalf("%s mismatch. Actual: %q, expected: %q", method, actual.String(), expected.String())
		}
	}

	const (
		user          = "user@EXAMPLE.COM"
		password      = "fakepw123"
		validConfig   = "[libdefaults]\nforwardable=false"
		invalidConfig = "[libdefaults]\nallow_weak_crypto=true"
	)

	k, err := kerberos.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Kerberos binding: ", err)
	}

	// Wipe any existing accounts from previous tests.
	clearResp, err := k.ClearAccounts(ctx)
	assertOk(err, clearResp.Error, "ClearAccounts")

	// Add an account.
	addResp, err := k.AddAccount(ctx, user)
	assertOk(err, addResp.Error, "AddAccount")

	// Set a valid config on the account.
	setConfigResp, err := k.SetConfig(ctx, user, validConfig)
	assertOk(err, setConfigResp.Error, "SetConfig")

	// Set an invalid config on the account.
	setConfigResp, err = k.SetConfig(ctx, user, invalidConfig)
	assertErrorEquals(err, setConfigResp.Error, kp.ErrorType_ERROR_BAD_CONFIG, "SetConfig")

	// Find out why the config was invalid.
	validateConfigResp, err := k.ValidateConfig(ctx, invalidConfig)
	assertErrorEquals(err, validateConfigResp.Error, kp.ErrorType_ERROR_BAD_CONFIG, "ValidateConfig")
	assertConfigErrorEquals(*validateConfigResp.ErrorInfo.Code, kp.ConfigErrorCode_CONFIG_ERROR_KEY_NOT_SUPPORTED, "Config error code")
	assertIntEquals(int(validateConfigResp.ErrorInfo.GetLineIndex()), 1, "Line index")

	// Acquire a Kerberos ticket.
	acquireKerberosTgtResp, err := k.AcquireKerberosTgt(ctx, user, password /*rememberPassword=*/, true /*useLoginPassword=*/, false)
	assertErrorEquals(err, acquireKerberosTgtResp.Error, kp.ErrorType_ERROR_CONTACTING_KDC_FAILED, "AcquireKerberosTgt")

	// List account and verify the data.
	listResp, err := k.ListAccounts(ctx)
	assertOk(err, listResp.Error, "ListAccounts")
	assertIntEquals(len(listResp.Accounts), 1, "Number of accounts")
	acc := listResp.Accounts[0]
	assertStringEquals(*acc.PrincipalName, user, "acc.PrincipalName")
	assertBoolEquals(*acc.IsManaged, false, "acc.IsManaged")
	assertBoolEquals(*acc.PasswordWasRemembered, true, "acc.PasswordWasRemembered")
	assertBoolEquals(*acc.UseLoginPassword, false, "acc.UseLoginPassword")
	assertStringEquals(string(acc.Krb5Conf), validConfig, "acc.Krb5conf")
	assertIntEquals(int(acc.GetTgtValiditySeconds()), 0, "acc.TgtValiditySeconds")
	assertIntEquals(int(acc.GetTgtRenewalSeconds()), 0, "acc.TgtRenewalSeconds")

	// Get files.
	getKerberosFilesResp, err := k.GetKerberosFiles(ctx, user)
	assertOk(err, getKerberosFilesResp.Error, "GetKerberosFiles")
	assertStringEquals(string(getKerberosFilesResp.Files.Krb5Conf), "", "getKerberosFilesResp.Files.Krb5conf")
	assertStringEquals(string(getKerberosFilesResp.Files.Krb5Cc), "", "getKerberosFilesResp.Files.Krb5Cc")

	// Remove account again.
	removeResp, err := k.RemoveAccount(ctx, user)
	assertOk(err, removeResp.Error, "RemoveAccount")

	// Verify that account list is empty.
	listResp, err = k.ListAccounts(ctx)
	assertOk(err, listResp.Error, "ListAccounts")
	assertIntEquals(len(listResp.Accounts), 0, "Number of accounts")
}
