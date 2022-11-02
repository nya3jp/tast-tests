// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	// Test to reproduce PKCS#11 slot ID change for shill.
	// The slot ID change is expected to only happen outside of a session.
	// This means that shill will always run its persistence logic.
	// This test expects that on such case, shill will load the correct ID.
	testing.AddTest(&testing.Test{
		Func: StableShillEAPSlot,
		Desc: "Test that shill automatically updates PKCS#11 slot IDs for EAP certificates",
		Contacts: []string{
			"jasongustaman@google.com",
			"cros-networking@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      7 * time.Minute,
	})
}

// slotT is the possible PKCS#11 slot types ChromeOS are using.
type slotT int

const (
	// systemSlot refers to the device-wide slot accessible across users.
	systemSlot slotT = iota
	// userSlot refers to the slot accessible to the current active user.
	userSlot
)

type stableSlotTestCase struct {
	ssid string
	slot slotT
	id   string
}

const (
	systemChapsAuth     = "000000"
	systemChapsPath     = "/var/lib/chaps"
	systemChapsLabel    = "System TPM Token"
	userChapsPrefixPath = "/run/daemon-store/chaps"
	tmpChapsAuth        = "1234"
	tmpChapsPath        = "/tmp/tmp_token"
)

func StableShillEAPSlot(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	c, err := netcertstore.CreateStore(ctx, hwsec.NewCmdRunner())
	if err != nil {
		s.Fatal("Failed to create cert store: ", err)
	}
	defer c.Cleanup(cleanupCtx)

	// Install private key and certificates in user slot.
	userID, err := c.InstallCertKeyPair(ctx, certificate.TestCert1().ClientCred.PrivateKey, certificate.TestCert1().ClientCred.Cert)
	if err != nil {
		s.Fatal("Failed to install CA cert: ", err)
	}
	// Install private key and certificates in system slot.
	systemID, cleanup, err := c.InstallSystemCertKeyPair(ctx, certificate.TestCert2().ClientCred.PrivateKey, certificate.TestCert2().ClientCred.Cert)
	if err != nil {
		s.Fatal("Failed to install CA cert: ", err)
	}
	defer cleanup(cleanupCtx)

	// Start Chrome.
	// This is needed for Chrome to know of the added keys and certificates
	// as the cert and key installation is done outside of NSS.
	// Without this, NetworkCertMigrator will delete the cert and key ID
	// shill property.
	// This also allows shill to get a configured user profile.
	cred := chrome.Creds{User: netcertstore.TestUsername, Pass: netcertstore.TestPassword}
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),     // to avoid resetings TPM
		chrome.FakeLogin(cred), // to use the same user as certs are installed for
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tcs := []stableSlotTestCase{
		stableSlotTestCase{
			ssid: "System SSID",
			slot: systemSlot,
			id:   systemID,
		},
		stableSlotTestCase{
			ssid: "User SSID",
			slot: userSlot,
			id:   userID,
		},
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Ensure we got the right global + user profile set up on login. Because shill profiles are pushed
	// asynchronously from login, we wait.
	profilePath := func(ctx context.Context) dbus.ObjectPath {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		watcher, err := m.CreateWatcher(ctx)
		if err != nil {
			s.Fatal("Failed to create watcher: ", err)
		}
		defer watcher.Close(ctx)

		for {
			paths, err := m.ProfilePaths(ctx)
			if err != nil {
				s.Fatal("Failed to get profile paths: ", err)
			}
			if len(paths) > 2 {
				s.Fatalf("Too many profiles: got %d, want 2", len(paths))
			}
			if len(paths) == 2 {
				// Last profile is the user profile.
				return paths[len(paths)-1]
			}
			// Fewer than 2? We may still be pushing the user profile; let's wait.
			if _, err := watcher.WaitAll(ctx, shillconst.ManagerPropertyProfiles); err != nil {
				s.Fatal("Failed to wait for user profile: ", err)
			}
		}
	}(ctx)
	// Configure shill services with certificate and key ID.
	// Limited properties are added as the services won't be connected.
	// The test only needs the service to be stored and then loaded.
	for _, tc := range tcs {
		id, err := getChapsPKCSID(ctx, tc, c)
		if err != nil {
			s.Fatal("Failed to get chaps PKCS#11 ID: ", err)
		}
		props := map[string]interface{}{
			shillconst.ServicePropertyType:      shillconst.TypeWifi,
			shillconst.ServicePropertySSID:      tc.ssid,
			shillconst.ServicePropertyEAPCertID: id,
			shillconst.ServicePropertyEAPKeyID:  id,
		}
		if _, err := m.ConfigureServiceForProfile(ctx, profilePath, props); err != nil {
			s.Fatal("Failed to configure shill service: ", err)
		}
	}
	if errs := comparePKCSIDs(ctx, tcs, c, m); len(errs) > 0 {
		s.Fatal("Mismatch between PKCS#11 ID of chaps and shill: ", errs)
	}

	s.Log("Restarting Shill to trigger persistence logic")
	// We lose connectivity when restarting Shill, and if that
	// races with the recover_duts network-recovery hooks, it may
	// interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	unlocked := false
	defer func() {
		if !unlocked {
			unlock()
		}
	}()
	// Stop shill to trigger the persistence logic.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}

	// Re-order tokens such that there is a mismatch between the slot IDs
	// stored by shill and the current tokens.
	cleanup, err = reorderTokens(ctx)
	if err != nil {
		s.Fatal("Failed re-order tokens: ", err)
	}
	defer cleanup(cleanupCtx)

	h, err := c.GetUserHash(ctx)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	userChapsPath := filepath.Join(userChapsPrefixPath, h)
	// Re-start Chrome. Chrome will re-load the user token.
	// This might result in the user token occupying slot 0.
	// Unload the token at the end of the test such that system token can
	// always occupy slot 0.
	defer func() {
		if err := testexec.CommandContext(ctx, "chaps_client", "--unload", "--path="+userChapsPath).Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to unload user token: ", err)
		}
	}()
	// This is needed for Chrome to know of the changes of the slot IDs,
	// avoiding any unwanted side-effect of NetworkCertMigrator.
	cr, err = chrome.New(
		ctx,
		chrome.KeepState(),     // to avoid resetings TPM
		chrome.FakeLogin(cred), // to use the same user as certs are installed for
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	cr.Close(cleanupCtx)

	// Re-start shill now that the tokens are re-ordered.
	// Shill's persistence logic should be able to fix mismatched slots.
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	unlock()
	unlocked = true

	m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Now that the slot IDs are re-ordered, shill storage will not have
	// the correct values. These values are corrected on shill storage load.
	// Expect shill to have the correct PKCS#11 ID related EAP properties.
	if errs := comparePKCSIDs(ctx, tcs, c, m); len(errs) > 0 {
		s.Fatal("Mismatch between PKCS#11 ID of chaps and shill: ", errs)
	}
}

// getChapsPKCSID gets shill PKCS#11 ID representation "SLOT_ID:OBJ_ID" of tc.
// The slot ID value is taken from chaps, netcertstore.Store.
func getChapsPKCSID(ctx context.Context, tc stableSlotTestCase, c *netcertstore.Store) (string, error) {
	switch tc.slot {
	case systemSlot:
		return fmt.Sprintf("%d:%s", c.SystemSlot, tc.id), nil
	case userSlot:
		return fmt.Sprintf("%d:%s", c.Slot, tc.id), nil
	default:
		return "", errors.Errorf("invalid slot type %v", tc.slot)
	}
}

// getShillPKCSID gets shill PKCS#11 ID representation "SLOT_ID:OBJ_ID" of tc.
// The PKCS#11 ID value is taken by from shill's service with matching SSID.
// Certificate ID and key ID are expected to be equal.
func getShillPKCSID(ctx context.Context, tc stableSlotTestCase, m *shill.Manager) (string, error) {
	props := map[string]interface{}{
		shillconst.ServicePropertyType: shillconst.TypeWifi,
		shillconst.ServicePropertyName: tc.ssid,
	}
	service, err := m.WaitForServiceProperties(ctx, props, 5*time.Second)
	if err != nil {
		return "", errors.Wrap(err, "failed to find shill service")
	}
	certID, err := service.GetEAPCertID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EAP certificate ID")
	}
	keyID, err := service.GetEAPKeyID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EAP key ID")
	}
	if certID != keyID {
		return "", errors.Errorf("certificate and key ID mismatch, got %s and %s", certID, keyID)
	}
	return certID, nil
}

// comparePKCSIDs compares shill PKCS#11 IDs with chaps PKCS#11 IDs.
// The function returns errors on failures or mismatches.
func comparePKCSIDs(ctx context.Context, tcs []stableSlotTestCase, c *netcertstore.Store, m *shill.Manager) (errs []error) {
	if err := c.UpdateTokenInfo(ctx); err != nil {
		return []error{errors.Wrap(err, "failed to update token info")}
	}
	for _, tc := range tcs {
		chapsID, err := getChapsPKCSID(ctx, tc, c)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to get chaps PKCS#11 ID"))
			continue
		}
		shillID, err := getShillPKCSID(ctx, tc, m)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to get shill PKCS#11 ID"))
			continue
		}
		// Confirm that the PKCS#11 ID is stored correctly.
		if shillID != chapsID {
			errs = append(errs, errors.Errorf("Stored PKCS#11 ID mismatch for %s, got: %s, want %s", tc.ssid, shillID, chapsID))
		}
	}
	return errs
}

// reorderTokens unloads and loads tokens resulting in PKCS#11 slot ID changes.
// There is no expectation for the resulting slot IDs values.
// The reorder is done by creating a temporary token in system token's slot.
// By doing so, the system token will be moved to use the next available slot,
// different from the system token's previous slot.
// The caller of this function is responsible to call the cleanup function.
func reorderTokens(ctx context.Context) (func(context.Context), error) {
	success := false
	cleanup := func(ctx context.Context) {
		// Unload temporary and system token.
		if err := testexec.CommandContext(ctx, "chaps_client", "--unload", "--path="+tmpChapsPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unload temporary token, most likely already removed by Chrome login: ", err)
		}
		if err := testexec.CommandContext(ctx, "chaps_client", "--unload", "--path="+systemChapsPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unload system token: ", err)
		}
		// Re-load system token to restore the original state.
		if err := testexec.CommandContext(ctx, "chaps_client", "--load", "--path="+systemChapsPath, "--auth="+systemChapsAuth, "--label="+systemChapsLabel).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to load system token: ", err)
		}
		// Remove the temporary directory.
		if err := testexec.CommandContext(ctx, "rm", "-rf", tmpChapsPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to delete tmp directory: ", err)
		}
	}
	defer func() {
		if !success {
			cleanup(ctx)
		}
	}()

	// Unload system token.
	if err := testexec.CommandContext(ctx, "chaps_client", "--unload", "--path="+systemChapsPath).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to unload system token")
	}
	// Load temporary token.
	// The new token will occupy slot 0 as the system token was unloaded.
	if err := testexec.CommandContext(ctx, "mkdir", "-p", tmpChapsPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to create a temporary directory")
	}
	if err := testexec.CommandContext(ctx, "chown", "chaps:chronos-access", tmpChapsPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to chown temporary directory")
	}
	if err := testexec.CommandContext(ctx, "chmod", "750", tmpChapsPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to chmod temporary directory")
	}
	if err := testexec.CommandContext(ctx, "chaps_client", "--load", "--path="+tmpChapsPath, "--auth="+tmpChapsAuth).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to load temporary token")
	}
	// Re-load system token.
	// On a normal condition, system token will now occupy slot 2.
	// This is because the temporary token and user token will occupy the
	// previous slots (0 and 1).
	if err := testexec.CommandContext(ctx, "chaps_client", "--load", "--path="+systemChapsPath, "--auth="+systemChapsAuth, "--label="+systemChapsLabel).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to re-load system token")
	}
	success = true
	return cleanup, nil
}
