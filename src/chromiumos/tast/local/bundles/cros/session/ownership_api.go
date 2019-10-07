// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/session/ownership"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OwnershipAPI,
		Desc: "Verifies that Ownership API works for a local device owner",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		Data: []string{"testcert.p12"},
	})
}

// deviceSetUp clears the ownership data of the DUT, then recreates the
// ownership data.
func deviceSetUp(ctx context.Context, user, pass, p12Path string, key *rsa.PublicKey) (err error) {
	const uiSetupTimeout = 90 * time.Second

	testing.ContextLog(ctx, "Restarting ui job")
	sctx, cancel := context.WithTimeout(ctx, uiSetupTimeout)
	defer cancel()

	if err = upstart.StopJob(sctx, "ui"); err != nil {
		return err
	}
	defer func() {
		// In case of error, run EnsureJobRunning with the original
		// context to recover the job for the following tests.
		if err == nil {
			return
		}
		// Ignore error.
		upstart.EnsureJobRunning(ctx, "ui")
	}()
	if err = session.ClearDeviceOwnership(sctx); err != nil {
		return err
	}
	if err = cryptohome.CreateVault(sctx, user, pass); err != nil {
		return err
	}
	if err = createOwnerKey(sctx, user, p12Path, key); err != nil {
		return err
	}
	err = upstart.EnsureJobRunning(sctx, "ui")
	return
}

// createOwnerKey creates ownership data of the DUT. Specifically, this
// pushes PKCS #12 certification data into NSS database, and creates
// /var/lib/whitelist/owner.key file.
func createOwnerKey(ctx context.Context, user, p12Path string, pubkey *rsa.PublicKey) error {
	testing.ContextLog(ctx, "Creating owner.key file")

	// Convert pubkey to PKIX DER form first, so that in case of error
	// happens, it won't be pushed to NSS.
	der, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		return errors.Wrap(err, "RSA public key cannot be marshaled into PKIX")
	}

	// Install p12 data into NSS database.
	if err = pushToNSS(ctx, user, p12Path); err != nil {
		return err
	}
	if err = setupOwnerKey(der); err != nil {
		return err
	}
	return nil
}

// pushToNSS installs PKCS #12 certification data into nssdb for the given user.
func pushToNSS(ctx context.Context, user, p12Path string) error {
	upath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return err
	}

	nssdb := filepath.Join(upath, ".pki", "nssdb")
	cmd := testexec.CommandContext(ctx, "pk12util", "-d", "sql:"+nssdb, "-i", p12Path, "-W", "" /* password */)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}

	return nil
}

// setupOwnerKey creates /var/lib/whitelist/owner.key file with given DER.
func setupOwnerKey(der []byte) error {
	// Ensure parent dir exists.
	const whitelistDir = "/var/lib/whitelist"
	if _, err := os.Stat(whitelistDir); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to stat %s", whitelistDir)
		}
		group, err := user.LookupGroup("policy-readers")
		if err != nil {
			return errors.Wrap(err, "policy-readers group not found")
		}
		gid, err := strconv.Atoi(group.Gid)
		if err != nil {
			return errors.Wrapf(err, "unexpected gid: %v", group)
		}

		// In order not to leave the intermediate dir on error,
		// create a temp directory, then rename.
		dir, err := ioutil.TempDir("/var/lib", ".whitelist.")
		if err != nil {
			return errors.Wrap(err, "failed to create a temp dir")
		}
		defer func() {
			if dir != "" {
				os.Remove(dir)
			}
		}()
		if err = os.Chmod(dir, 0750); err != nil {
			return errors.Wrap(err, "failed to set permission")
		}
		if err = os.Chown(dir, -1, gid); err != nil {
			return errors.Wrap(err, "failed to chown")
		}
		if err = os.Rename(dir, whitelistDir); err != nil {
			return errors.Wrap(err, "failed to rename a temporaly dir")
		}
		dir = "" // Not to fire os.Remove(dir) in defer.
	}

	ownerKeyPath := filepath.Join(whitelistDir, "owner.key")
	if err := ioutil.WriteFile(ownerKeyPath, der, 0604); err != nil {
		return errors.Wrapf(err, "failed to write to %s", ownerKeyPath)
	}
	return nil
}

// sign signs the blob with the given key, and returns the signature.
func sign(key *rsa.PrivateKey, blob []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(blob)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(nil, key, crypto.SHA1, digest)
}

func OwnershipAPI(ctx context.Context, s *testing.State) {
	const (
		testUser = "ownership_test@chromium.org"
		testPass = "testme"
	)

	p12Path := s.DataPath("testcert.p12")
	privKey, err := session.ExtractPrivKey(p12Path)
	if err != nil {
		s.Fatal("Failed to parse PKCS #12 file: ", err)
	}
	if err := deviceSetUp(ctx, testUser, testPass, p12Path, &privKey.PublicKey); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	if err = sm.StartSession(ctx, testUser, ""); err != nil {
		s.Fatalf("Failed to start new session for %s: %v", testUser, err)
	}

	settings := ownership.BuildTestSettings(testUser)
	if err := session.StoreSettings(ctx, sm, testUser, privKey, nil, settings); err != nil {
		s.Fatal("Failed to store settings data: ", err)
	}

	// Fetch the data from the session_manager.
	ret, err := session.RetrieveSettings(ctx, sm)
	if err != nil {
		s.Fatal("Failed to retrieve settings: ", err)
	}

	// Verify that there's no diff between sent data and fetched data.
	if diff := cmp.Diff(settings, ret); diff != "" {
		const diffName = "diff.txt"
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), diffName), []byte(diff), 0644); err != nil {
			s.Error("Failed to write diff: ", err)
		}
		s.Error("Sent data and fetched data has diff, which is found in ", diffName)
	}
}
