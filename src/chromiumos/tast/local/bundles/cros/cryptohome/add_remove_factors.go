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

	uda "chromiumos/system_api/user_data_auth_proto"
	cryptohomecommon "chromiumos/tast/common/cryptohome"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

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
			Name:    "with_vk",
			Fixture: "vkAuthSessionFixture",
		}, {
			Name:    "with_uss",
			Fixture: "ussAuthSessionFixture",
		}},
	})
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

	fixture := s.FixtValue().(*cryptohome.AuthSessionFixture)
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
	supportsPIN, err := client.SupportsLECredentials(ctx)
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
	var expectedOnlyPassword = []*uda.AuthFactorWithStatus{{
		AuthFactor: &uda.AuthFactor{
			Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
			Label: passwordLabel,
		},
	}}
	expectedConfiguredFactors := expectedOnlyPassword

	// Some individual factors for types that might be included, depending on the hardware.
	var expectedPIN = &uda.AuthFactorWithStatus{
		AuthFactor: &uda.AuthFactor{
			Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN,
			Label: pinLabel,
		},
	}
	var expectedSmartCard = &uda.AuthFactorWithStatus{
		AuthFactor: &uda.AuthFactor{
			Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD,
			Label: smartCardLabel,
		},
	}

	// The final set of auth factors at the end of the test depends on whether or not PIN is supported.
	expectedFinalConfiguredFactors := expectedOnlyPassword
	if supportsPIN {
		expectedFinalConfiguredFactors = append(expectedFinalConfiguredFactors, expectedPIN)
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
	if supportsPIN {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN)
	}
	if supportsSmartCard {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_SMART_CARD)
	}
	if fixture.UssEnabled {
		expectedAllSupported = append(expectedAllSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_CRYPTOHOME_RECOVERY)
		expectedNoKioskSupported = append(expectedNoKioskSupported, uda.AuthFactorType_AUTH_FACTOR_TYPE_CRYPTOHOME_RECOVERY)
	}

	// List the auth factors for the user. There should be no factors, configured, an maximum factors supported.
	listFactorsAtStartReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors before adding any factors: ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactorsAtStartReply.ConfiguredAuthFactorsWithStatus, nil); err != nil {
		s.Fatal("Mismatch in configured auth factors before adding factors (-got, +want): ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorTypes(
		listFactorsAtStartReply.SupportedAuthFactors, expectedAllSupported); err != nil {
		s.Fatal("Mismatch in supported auth factors before adding factors (-got, +want): ", err)
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add password auth factor: ", err)
	}

	// List the auth factors for the user. There should be only password.
	listFactorsAfterAddPasswordReply, err := client.ListAuthFactors(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list auth factors after adding password: ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactorsAfterAddPasswordReply.ConfiguredAuthFactorsWithStatus, expectedOnlyPassword); err != nil {
		s.Fatal("Mismatch in configured auth factors after adding password (-got, +want): ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorTypes(
		listFactorsAfterAddPasswordReply.SupportedAuthFactors, expectedNoKioskSupported); err != nil {
		s.Fatal("Mismatch in supported auth factors after adding password (-got, +want): ", err)
	}

	if supportsPIN {
		// Add a PIN auth factor.
		if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
			s.Fatal("Failed to add PIN auth factor: ", err)
		}

		// Update configured auth factors base on what we expect to see.
		expectedConfiguredFactors = append(expectedConfiguredFactors, expectedPIN)

		// List the auth factors for the user. There should be password and pin.
		listFactorsAfterAddPinReply, err := client.ListAuthFactors(ctx, userName)
		if err != nil {
			s.Fatal("Failed to list auth factors after adding PIN: ", err)
		}
		if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
			listFactorsAfterAddPinReply.ConfiguredAuthFactorsWithStatus, expectedConfiguredFactors); err != nil {
			s.Fatal("Mismatch in configured auth factors after adding PIN (-got, +want): ", err)
		}
		if err := cryptohomecommon.ExpectAuthFactorTypes(
			listFactorsAfterAddPinReply.SupportedAuthFactors, expectedNoKioskSupported); err != nil {
			s.Fatal("Mismatch in supported auth factors after adding PIN (-got, +want): ", err)
		}
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
		if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
			listFactorsAfterAddSmartCardReply.ConfiguredAuthFactorsWithStatus, expectedConfiguredFactors); err != nil {
			s.Fatal("Mismatch in configured auth factors after adding Smart Card (-got, +want): ", err)
		}
		if err := cryptohomecommon.ExpectAuthFactorTypes(
			listFactorsAfterAddSmartCardReply.SupportedAuthFactors, expectedNoKioskSupported); err != nil {
			s.Fatal("Mismatch in supported auth factors after adding Smart Card (-got, +want): ", err)
		}
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
	if err := cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactorsAfterUnmount.ConfiguredAuthFactorsWithStatus, expectedFinalConfiguredFactors); err != nil {
		s.Fatal("Mismatch in configured auth factors after adding unmount (-got, +want): ", err)
	}
	if err := cryptohomecommon.ExpectAuthFactorTypes(
		listFactorsAfterUnmount.SupportedAuthFactors, expectedNoKioskSupported); err != nil {
		s.Fatal("Mismatch in supported auth factors after adding unmount (-got, +want): ", err)
	}
}
