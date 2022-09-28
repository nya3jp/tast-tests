// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// checkKeyPerfParam contains the test parameters which are different
// between the types of CheckKeyPerf test.
type checkKeyPerfParam struct {
	// Specifies which mount flow to use - AuthSession+Split Call or legacy MountEx.
	legacyMountFlow bool
	// Specifies the name of metric to log checkKey duration with.
	checkKeyDurationMetric string
	// Specifies the name of metric to log checkKey with WebAuthn duration with.
	checkKeyDurationWithWebAuthnMetric string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckKeyPerf,
		Desc: "Performance for CheckKey operation",
		Contacts: []string{
			"dlunev@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"hwsec_destructive_crosbolt_perbuild", "group:hwsec_destructive_crosbolt"},
		SoftwareDeps: []string{"tpm", "reboot"},
		Vars:         []string{"hwsec.CheckKeyPerf.iterations"},
		Params: []testing.Param{{
			Name: "legacy_mountex",
			Val: checkKeyPerfParam{
				legacyMountFlow:                    true,
				checkKeyDurationMetric:             "check_key_ex_duration",
				checkKeyDurationWithWebAuthnMetric: "check_key_ex_unlock_webauthn_secret_duration",
			},
		}, {
			Name: "auth_session_split_mount",
			Val: checkKeyPerfParam{
				legacyMountFlow:                    false,
				checkKeyDurationMetric:             "auth_session_setup_check_key_ex_duration",
				checkKeyDurationWithWebAuthnMetric: "auth_session_setup_check_key_ex_unlock_webauthn_secret_duration",
			},
		}},
	})
}

func CheckKeyPerf(ctx context.Context, s *testing.State) {
	// Setup helper functions
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()
	userParam := s.Param().(checkKeyPerfParam)

	// Reset TPM
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	if err := setupUser(ctx, userParam.legacyMountFlow, helper); err != nil {
		s.Fatal("Failed to setup user: ", err)
	}

	// Cleanup upon finishing
	defer func() {
		if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}()

	// Get iterations count from the variable or default it.
	iterations := int64(50)
	if val, ok := s.Var("hwsec.CheckKeyPerf.iterations"); ok {
		var err error
		iterations, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			s.Fatal("Unparsable iterations variable: ", err)
		}
	}

	value := perf.NewValues()

	// Run |iterations| times CheckKeyEx.
	for i := int64(0); i < iterations; i++ {
		startTs := time.Now()
		result, err := utility.CheckVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1))
		duration := time.Now().Sub(startTs)
		if err != nil {
			s.Fatal("Call to CheckKeyEx with the correct username and password resulted in an error: ", err)
		}
		if !result {
			s.Fatal("Failed to CheckKeyEx() with the correct username and password: ", err)
		}
		value.Append(perf.Metric{
			Name:      userParam.checkKeyDurationMetric,
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))
	}

	// Run |iterations| times CheckKeyEx with unlocking webauthn secret.
	for i := int64(0); i < iterations; i++ {
		startTs := time.Now()
		result, err := utility.CheckVaultAndUnlockWebAuthnSecret(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1))
		duration := time.Now().Sub(startTs)
		if err != nil {
			s.Fatal("Call to CheckKeyEx (with unlocking webauthn secret) with the correct username and password resulted in an error: ", err)
		}
		if !result {
			s.Fatal("Failed to CheckKeyEx() (with unlocking webauthn secret) with the correct username and password: ", err)
		}
		value.Append(perf.Metric{
			Name:      userParam.checkKeyDurationWithWebAuthnMetric,
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))
	}

	if err := value.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf-results: ", err)
	}
}

func setupUser(ctx context.Context, useLegacyMountFlow bool, helper *hwsecremote.CmdHelperRemote) error {
	utility := helper.CryptohomeClient()

	if useLegacyMountFlow {
		// Create and Mount vault.
		createVault := true
		if err := utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), createVault, hwsec.NewVaultConfig()); err != nil {
			return errors.Wrap(err, "failed to create user")
		}
	} else {
		// Start an Auth session and get an authSessionID.
		isEphemeral := false
		_, authSessionID, err := utility.StartAuthSession(ctx, util.FirstUsername, isEphemeral, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer utility.InvalidateAuthSession(ctx, authSessionID)

		if err := utility.CreatePersistentUser(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to create persistent user")
		}

		if err := utility.PreparePersistentVault(ctx, authSessionID, false); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}

		isKioskUser := false
		if err := utility.AddCredentialsWithAuthSession(ctx, util.FirstUsername, util.FirstPassword1, util.Password1Label, authSessionID, isKioskUser); err != nil {
			return errors.Wrap(err, "failed to add credentials with AuthSession")
		}
	}
	return nil
}
