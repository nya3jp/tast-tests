// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// SetConsent sets up the system to have, or not to have metrics consent. Note
// that if you use chrome.New() after this, you must use chrome.KeepState() as
// one of the parameters. certFile should be the complete path to a PKCS #12
// format file.
func SetConsent(ctx context.Context, certFile string, consent bool) error {
	const (
		legacyConsent = "/home/chronos/Consent To Send Stats"
	)

	testing.ContextLog(ctx, "Setting up consent")
	privKey, err := session.ExtractPrivKey(certFile)
	if err != nil {
		return errors.Wrap(err, "failed to parse PKCS #12 file")
	}
	settings := &enterprise_management.ChromeDeviceSettingsProto{
		MetricsEnabled: &enterprise_management.MetricsEnabledProto{
			MetricsEnabled: proto.Bool(consent),
		},
	}
	if err := session.SetUpDevice(ctx); err != nil {
		return errors.Wrap(err, "failed to reset device ownership")
	}
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "could not bind to session manager")
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		return errors.Wrap(err, "failed to prepare Chrome for testing")
	}
	// Create clean vault for the test user, and start the session.
	if err := cryptohome.RemoveVault(ctx, chrome.DefaultUser); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	if err := cryptohome.CreateVault(ctx, chrome.DefaultUser, chrome.DefaultPass); err != nil {
		return errors.Wrap(err, "failed to create vault")
	}
	if err := session.StoreSettings(ctx, sm, chrome.DefaultUser, privKey, nil, settings); err != nil {
		return errors.Wrap(err, "failed to store user policy")
	}
	if consent {
		// Create deprecated consent file.  This is created *after* the
		// policy file in order to avoid a race condition where Chrome
		// might remove the consent file if the policy's not set yet.
		// We create it as a temp file first in order to make the creation
		// of the consent file, owned by chronos, atomic.
		// See crosbug.com/18413.
		tempFile := legacyConsent + ".tmp"
		if err := ioutil.WriteFile(tempFile, []byte("test-consent"), 0644); err != nil {
			return errors.Wrapf(err, "failed to write to legacy consent file %s", tempFile)
		}

		if err := os.Chown(tempFile, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			return errors.Wrapf(err, "failed to chown legacy consent file %s", tempFile)
		}
		if err := os.Rename(tempFile, legacyConsent); err != nil {
			return errors.Wrapf(err, "failed to rename legacy consent file %s to %s", tempFile, legacyConsent)
		}
	} else {
		os.Remove(legacyConsent)
	}

	return nil
}
