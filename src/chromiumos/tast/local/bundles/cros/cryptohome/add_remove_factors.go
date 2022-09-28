// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"math/rand"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
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
	Type  uda.AuthFactorType
	Label string
}

// compareReplyToExpectations will compare the reply from ListAuthFactors to a
// set of expected factors. The comparison will check both configured factors
// (specified by type and label only) as well as the supported factors.
//
// The `when` parameter should be a string that can be included in the error
// messages describing when this expectation was checked. It should generally
// look something like "before xyz" or "after abc".
func compareReplyToExpectations(when string, reply *uda.ListAuthFactorsReply, expectedConfigured []expectedConfiguredFactor, expectedSupported []uda.AuthFactorType, s *testing.State) {
	// Compare the configured and expected configured factors. We do this by
	// reducing the configured factors in the reply to a list of
	// expectedConfiguredFactor instances, so that we can do a direct diff of
	// the two lists (actual vs expected). Order does not matter.
	if len(reply.GetConfiguredAuthFactorsWithStatus()) != len(expectedConfigured) {
		s.Fatalf("ListAuthFactors reported the wrong number of factors (got %d, want %d) %s", len(reply.GetConfiguredAuthFactorsWithStatus()), len(expectedConfigured), when)
	}
	actualConfigured := make([]expectedConfiguredFactor, 0, len(reply.ConfiguredAuthFactorsWithStatus))
	for _, configured := range reply.ConfiguredAuthFactorsWithStatus {
		newConfigured := expectedConfiguredFactor{
			Type:  configured.AuthFactor.Type,
			Label: configured.AuthFactor.Label,
		}
		actualConfigured = append(actualConfigured, newConfigured)
	}
	configuredLess := func(a, b expectedConfiguredFactor) bool {
		return a.Type < b.Type || (a.Type == b.Type && a.Label < b.Label)
	}
	if diff := cmp.Diff(actualConfigured, expectedConfigured, cmpopts.SortSlices(configuredLess)); diff != "" {
		s.Errorf("Mismatch in configured auth factors %s (-got +want) %s", when, diff)
	}
	// Compare the supported and expected supports factors. Order does not matter.
	typeLess := func(a, b uda.AuthFactorType) bool { return a < b }
	if diff := cmp.Diff(reply.SupportedAuthFactors, expectedSupported, cmpopts.SortSlices(typeLess)); diff != "" {
		s.Errorf("Mismatch in supported auth factors %s (-got +want) %s", when, diff)
	}
}

func AddRemoveFactors(ctx context.Context, s *testing.State) {
	const (
		userName       = "foo@bar.baz"
		userPassword   = "secret"
		passwordLabel  = "online-password"
		userPin        = "12345"
		pinLabel       = "luggage-pin"
		smartCardLabel = "smart-card-label"
		dbusName       = "org.chromium.TestingCryptohomeKeyDelegate"
		keySizeBits    = 2048
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

	// Determine if the client supports using Smart Card auth. EnsureTPMIsReady
	// will fetch the status of the TPM if it exists, and on error it is
	// assmued that device does not support TPM operations. An error here should not
	// cause the test to fail.
	supportsSmartCard := false
	authConfig := (*hwsec.AuthConfig)(nil)
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err == nil {
		supportsSmartCard = true

		// Set up KeyDelegate for the Smart Card.
		// Use a pseudorandom generator with a fixed seed, to make the values used by
		// the test predictable.
		randReader := rand.New(rand.NewSource(0 /* seed */))

		rsaKey, err := rsa.GenerateKey(randReader, keySizeBits)
		if err != nil {
			s.Fatal("Failed to generate RSA key: ", err)
		}
		pubKeySPKIDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
		if err != nil {
			s.Fatal("Failed to generate SubjectPublicKeyInfo: ", err)
		}

		dbusConn, err := dbusutil.SystemBus()
		if err != nil {
			s.Fatal("Failed to connect to system D-Bus bus: ", err)
		}
		if _, err := dbusConn.RequestName(dbusName, 0 /* flags */); err != nil {
			s.Fatal("Failed to request the well-known D-Bus name: ", err)
		}
		defer dbusConn.ReleaseName(dbusName)

		keyDelegate, err := hwsec.NewCryptohomeKeyDelegate(
			s.Logf, dbusConn, userName, hwsec.SmartCardAlgorithms, rsaKey, pubKeySPKIDER)
		if err != nil {
			s.Fatal("Failed to export D-Bus key delegate: ", err)
		}
		defer keyDelegate.Close()

		// Prepare Smart Card config.
		authConfig = hwsec.NewChallengeAuthConfig(userName, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, hwsec.SmartCardAlgorithms)
	}

	// Enable the UserSecretStash experiment if USS is specified.
	if userParam.useUserSecretStash {
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctx)
	}

	// Create and mount the persistent user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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
	var expectedPin = expectedConfiguredFactor{uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN, pinLabel}
	var expectedSmartCard = expectedConfiguredFactor{uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD, smartCardLabel}

	// The final set of auth factors at the end of the test depends on whether or not PIN is supported.
	expectedFinalConfiguredFactors := expectedOnlyPassword
	expectedConfiguredFactors := expectedOnlyPassword
	if supportsPin {
		expectedFinalConfiguredFactors = append(expectedFinalConfiguredFactors, expectedPin)
	}

	if supportsSmartCard {
		expectedFinalConfiguredFactors = append(expectedFinalConfiguredFactors, expectedSmartCard)
	}

	// Expected supported auth factors at different points in the test.
	var expectedAllSupported = []uda.AuthFactorType{
		uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
		uda.AuthFactorType_AUTH_FACTOR_TYPE_KIOSK,
	}
	var expectedNoKioskSupported = []uda.AuthFactorType{
		uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
	}
	// The supported lists may also need PIN, RECOVERY or SMART_CARD, depending on the DUT and the test params.
	if supportsPin {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
	}
	if supportsSmartCard {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD)
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

	// List the auth factors for the user. There should be only password.
	listFactorsAfterAddPasswordReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after adding password: ", err)
	}
	compareReplyToExpectations("after adding password", listFactorsAfterAddPasswordReply, expectedOnlyPassword, expectedNoKioskSupported, s)

	if supportsPin {
		// Add a PIN auth factor.
		if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
			s.Fatal("Failed to add PIN auth factor: ", err)
		}

		// Update configured auth factors base on what we expect to see.
		expectedConfiguredFactors = append(expectedConfiguredFactors, expectedPin)

		// List the auth factors for the user. There should be password and pin.
		listFactorsAfterAddPinReply, err := client.ListAuthFactors(ctx, userName)
		if err != nil {
			s.Fatal("Failed to list auth factors after adding PIN: ", err)
		}
		compareReplyToExpectations("after adding PIN", listFactorsAfterAddPinReply, expectedConfiguredFactors, expectedNoKioskSupported, s)
	}

	// TODO(b/241016536) Smart Cards implementation only works with VaultKeyset, USS will be implemented later.
	if supportsSmartCard {
		// Add a Smart Card auth factor.
		if err := client.AddSmartCardAuthFactor(ctx, authSessionID, smartCardLabel, authConfig); err != nil {
			s.Fatal("Failed to add Smart Card auth factor: ", err)
		}

		// Update configured auth factors we expect to see.
		expectedConfiguredFactors = append(expectedConfiguredFactors, expectedSmartCard)

		// List the auth factors for the user. There should be password and pin.
		listFactorsAfterAddSmartCardReply, err := client.ListAuthFactors(ctx, userName)
		if err != nil {
			s.Fatal("Failed to list auth factors after adding Smart Card: ", err)
		}
		compareReplyToExpectations("after adding Smart Card", listFactorsAfterAddSmartCardReply, expectedConfiguredFactors, expectedNoKioskSupported, s)
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
	compareReplyToExpectations("after unmount", listFactorsAfterUnmount, expectedFinalConfiguredFactors, expectedNoKioskSupported, s)
}
