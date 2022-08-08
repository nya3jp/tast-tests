// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// Parameteters that control test behavior.
type addRemoveFactorsParams struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AddRemoveFactors,
		Desc: "Test adding, removing, and listing auth factors",
		Contacts: []string{
			"jadmanski@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "add_remove_factors_with_vk",
			Val: addRemoveFactorsParams{
				useUserSecretStash: false,
			},
		}, {
			Name: "add_remove_factors_with_uss",
			Val: addRemoveFactorsParams{
				useUserSecretStash: true,
			},
		}},
	})
}

func AddRemoveFactors(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		passwordLabel = "online-password"
	)

	userParam := s.Param().(addRemoveFactorsParams)
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

	// Enable the UserSecretStash experiment if USS is specified.
	if userParam.useUserSecretStash {
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment()
	}

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// List the auth factors for the user. There should be none.
	listFactorsZeroFactorsReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors: ", err)
	}
	if len(listFactorsZeroFactorsReply.GetConfiguredAuthFactors()) != 0 {
		s.Fatal("ListAuthFactors reported auth factors before any were added")
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	// List the auth factors for the user. There should be none.
	listFactorsOneFactorReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors: ", err)
	}
	if len(listFactorsOneFactorReply.GetConfiguredAuthFactors()) == 0 {
		s.Fatal("ListAuthFactors reported no factors even after adding one")
	} else if len(listFactorsOneFactorReply.GetConfiguredAuthFactors()) > 1 {
		s.Fatal("ListAuthFactors reported multiple factors but we only added one")
	}
	if listFactorsOneFactorReply.ConfiguredAuthFactors[0].Type != uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD {
		s.Fatalf("Added auth factor does not have the correct type: got %d; want %d", listFactorsOneFactorReply.ConfiguredAuthFactors[0].Type, uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD)
	}
	if listFactorsOneFactorReply.ConfiguredAuthFactors[0].Label != passwordLabel {
		s.Fatalf("Added auth factor does not have the correct label: got %q; want %q", listFactorsOneFactorReply.ConfiguredAuthFactors[0].Label, passwordLabel)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// Even after unmount we should still be able to list the auth factors.
	listFactorsAfterUnmount, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after unmount: ", err)
	}
	if len(listFactorsAfterUnmount.GetConfiguredAuthFactors()) == 0 {
		s.Fatal("ListAuthFactors reported no factors after unmount")
	} else if len(listFactorsAfterUnmount.GetConfiguredAuthFactors()) > 1 {
		s.Fatal("ListAuthFactors reported multiple factors after unmount")
	}
	if listFactorsAfterUnmount.ConfiguredAuthFactors[0].Type != uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD {
		s.Fatalf("After unmount auth factor does not have the correct type: got %d; want %d", listFactorsAfterUnmount.ConfiguredAuthFactors[0].Type, uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD)
	}
	if listFactorsAfterUnmount.ConfiguredAuthFactors[0].Label != passwordLabel {
		s.Fatalf("After unmount auth factor does not have the correct label: got %q; want %q", listFactorsAfterUnmount.ConfiguredAuthFactors[0].Label, passwordLabel)
	}
}
