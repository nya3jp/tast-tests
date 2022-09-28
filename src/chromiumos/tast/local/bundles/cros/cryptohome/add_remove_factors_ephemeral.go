// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	cryptohomecommon "chromiumos/tast/common/cryptohome"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// Parameteters that control test behavior.
type addRemoveFactorsEphemeralParams struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AddRemoveFactorsEphemeral,
		Desc: "Test adding, removing, and listing auth factors for ephemeral users",
		Contacts: []string{
			"jadmanski@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{"testcert.p12"},
		// While the ephemeral user behavior shouldn't in theory be affected by
		// whether the UserSecretStash is enabled in cryptohome, we have two
		// separate sub-tests to actually verify this in both cases.
		Params: []testing.Param{{
			Name: "with_vk",
			Val: addRemoveFactorsEphemeralParams{
				useUserSecretStash: false,
			},
		}, {
			Name: "with_uss",
			Val: addRemoveFactorsEphemeralParams{
				useUserSecretStash: true,
			},
		}},
	})
}

func AddRemoveFactorsEphemeral(ctx context.Context, s *testing.State) {
	const (
		ownerName     = "owner@bar.baz"
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		passwordLabel = "online-password"
		userPin       = "12345"
		pinLabel      = "luggage-pin"
	)

	userParam := s.Param().(addRemoveFactorsEphemeralParams)
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Set up an owner. This is needed for ephemeral users.
	if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerName, "whatever", "whatever", helper.CryptohomeClient()); err != nil {
		client.UnmountAll(ctx)
		client.RemoveVault(ctx, ownerName)
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	defer client.RemoveVault(ctxForCleanUp, ownerName)

	// Enable the UserSecretStash experiment if USS is specified.
	if userParam.useUserSecretStash {
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctx)
	}

	// Create and mount the ephemeral user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, true, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare new ephemeral vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// List the auth factors before we've added any factors.
	listFactorsAtStartReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors before adding any factors: ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactorsAtStartReply.ConfiguredAuthFactorsWithStatus,
		nil); err != nil {
		s.Fatal("Mismatch in configured auth factors before adding factors (-got, +want): ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorTypes(
		listFactorsAtStartReply.SupportedAuthFactors,
		[]uda.AuthFactorType{uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD}); err != nil {
		s.Fatal("Mismatch in supported auth factors before adding factors (-got, +want): ", err)
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add password auth factor: ", err)
	}

	// List the auth factors for the user now that we've added a password factor.
	listFactorsAfterAddPasswordReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after adding password: ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactorsAfterAddPasswordReply.ConfiguredAuthFactorsWithStatus,
		[]*uda.AuthFactorWithStatus{{
			AuthFactor: &uda.AuthFactor{
				Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
				Label: passwordLabel,
			},
		}}); err != nil {
		s.Fatal("Mismatch in configured auth factors after adding password (-got, +want): ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorTypes(
		listFactorsAfterAddPasswordReply.SupportedAuthFactors,
		[]uda.AuthFactorType{uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD}); err != nil {
		s.Fatal("Mismatch in supported auth factors after adding password (-got, +want): ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// After unmount listing auth factors should fail.
	if _, err := client.ListAuthFactors(ctx, userName); err == nil {
		s.Fatal("Unexpectedly succeeded at listing auth factors after unmount")
	}
}
