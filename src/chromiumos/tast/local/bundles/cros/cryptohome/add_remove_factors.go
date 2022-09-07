// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
			Name: "with_vk",
			Val: addRemoveFactorsParams{
				useUserSecretStash: false,
			},
		}, {
			Name: "with_uss",
			Val: addRemoveFactorsParams{
				useUserSecretStash: true,
			},
		}},
	})
}

// Structure for specifying the expected fields in an AuthFactor.
type expectedConfiguredFactor struct {
	factorType uda.AuthFactorType
	label      string
}

// compareReplyToExpectations will compare the reply from ListAuthFactors to a set of expected factors.
// The comparison will check both configured factors (specified by type and label only) as well as the
// supported factors.
//
// The `when` parameter should be a string that can be included in the error messages describing when
// this expectation was checked. It should generally look something like "before xyz" or "after abc".
func compareReplyToExpectations(when string, reply *uda.ListAuthFactorsReply, expectedConfigured []expectedConfiguredFactor, expectedSupported []uda.AuthFactorType, s *testing.State) {
	// Compare the configured and expected configured factors. Currently order matters, but it might make
	// sense to change that in the future as we configure more factors in the test.
	if len(reply.GetConfiguredAuthFactors()) != len(expectedConfigured) {
		s.Fatalf("ListAuthFactors reported the wrong number of factors (got %d, want %d) %s", len(reply.GetConfiguredAuthFactors()), len(expectedConfigured), when)
	}
	for i, expected := range expectedConfigured {
		factor := reply.ConfiguredAuthFactors[i]
		if factor.Type != expected.factorType {
			s.Fatalf("Auth factor %d does not have the correct type (got %d, want %d) %s", i, factor.Type, expected.factorType, when)
		}
		if factor.Label != expected.label {
			s.Fatalf("Auth factor %d does not have the correct label (got %q, want %q) %s", i, factor.Label, expected.label, when)
		}
	}
	// Compare the supported and expected supports factors. Order does not matter.
	typeLess := func(a, b uda.AuthFactorType) bool { return a < b }
	if diff := cmp.Diff(reply.SupportedAuthFactors, expectedSupported, cmpopts.SortSlices(typeLess)); diff != "" {
		s.Errorf("Mismatch in supported auth factors %s (-got +want) %s", when, diff)
	}
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

	// Determine if the client supports using PIN auth.
	supportsPin, err := client.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Unable to determine if PINs are supported: ", err)
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

	// Expected configured auth factors at different points in the test.
	var expectedNoFactors = []expectedConfiguredFactor{}
	var expectedOnlyPassword = []expectedConfiguredFactor{
		{uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD, passwordLabel},
	}

	// Expected supported auth factors at different points in the test.
	var expectedAllSupported = []uda.AuthFactorType{
		uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
		uda.AuthFactorType_AUTH_FACTOR_TYPE_KIOSK,
		uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD,
	}
	var expectedNoKioskSupported = []uda.AuthFactorType{
		uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
		uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD,
	}
	// The supported lists may also need PIN or RECOVERY, depending on the DUT and the test params.
	if supportsPin {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
	}
	if userParam.useUserSecretStash {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_CRYPTOHOME_RECOVERY)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_CRYPTOHOME_RECOVERY)
	}

	// List the auth factors for the user. There should be no factors, configured, an maximum factors supported.
	listFactorsAtStartReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors before adding any factors: ", err)
	}
	compareReplyToExpectations("before adding factors", listFactorsAtStartReply, expectedNoFactors, expectedAllSupported, s)

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add password auth factor: ", err)
	}

	// List the auth factors for the user. There should be none.
	listFactorsAfterAddPasswordReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after adding password: ", err)
	}
	compareReplyToExpectations("after adding password", listFactorsAfterAddPasswordReply, expectedOnlyPassword, expectedNoKioskSupported, s)

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// Even after unmount we should still be able to list the auth factors.
	listFactorsAfterUnmount, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after unmount: ", err)
	}
	compareReplyToExpectations("after unmount", listFactorsAfterUnmount, expectedOnlyPassword, expectedNoKioskSupported, s)
}
