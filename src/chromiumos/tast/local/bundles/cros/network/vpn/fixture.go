// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const certOpTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vpnShillReset",
		Desc: "A fixture that sets up the environment for VPN connections, including resetting shill and installing certs",
		Contacts: []string{
			"jiejiang@google.com",        // fixture maintainer
			"cros-networking@google.com", // platform networking team
		},
		SetUpTimeout:    shill.ResetShillTimeout + certOpTimeout + 5*time.Second,
		ResetTimeout:    shill.ResetShillTimeout + 5*time.Second,
		TearDownTimeout: shill.ResetShillTimeout + certOpTimeout + 5*time.Second,
		Impl:            &vpnFixture{useCr: false},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "vpnShillResetWithChromeLoggedIn",
		Desc: "A fixture that sets up the environment for VPN connections, including resetting shill, installing certs, and starting Chrome session",
		Contacts: []string{
			"jiejiang@google.com",        // fixture maintainer
			"cros-networking@google.com", // platform networking team
		},
		SetUpTimeout:    shill.ResetShillTimeout + certOpTimeout + chrome.LoginTimeout + 5*time.Second,
		ResetTimeout:    shill.ResetShillTimeout + chrome.ResetTimeout + 5*time.Second,
		TearDownTimeout: shill.ResetShillTimeout + certOpTimeout + chrome.LoginTimeout + 5*time.Second,
		Impl:            &vpnFixture{useCr: true},
	})
}

func resetShillWithLockingHook(ctx context.Context) error {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us. Lock the hook
	// before shill restarted.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lock check network hook")
	}
	defer unlock()

	if errs := shill.ResetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			testing.ContextLog(ctx, "ResetShill error: ", err)
		}
		return errors.Wrap(errs[0], "failed to reset shill")
	}

	return nil
}

// vpnFixture is a fixture to prepare environment that can be used to test VPN
// connections. Particularly, this fixture does the followings:
// - Reset shill in SetUp and TearDown, to make sure we have a clean shill profile.
// - Prepare the cert store and install user certificate (and server CA certificate
//	 if Chrome is required).
// - Start a new Chrome session if required.
// When a test failed, to ensure we have a clean setup, shill will be reset if
// there is no Chrome, and a full restart of this fixture will happen if there is Chrome.
type vpnFixture struct {
	hasError  bool // if the previous test has error
	useCr     bool // if Chrome is needed
	cr        *chrome.Chrome
	certStore *netcertstore.Store
}

// FixtureEnv wraps the variables created by the fixture and used in the tests.
type FixtureEnv struct {
	Cr       *chrome.Chrome
	CertVals CertVals
}

// CertVals contains the required values to setup a cert-based VPN service.
type CertVals struct {
	id   string
	slot string
	pin  string
}

func installUserCert(ctx context.Context, certStore *netcertstore.Store) (CertVals, error) {
	slot := fmt.Sprintf("%d", certStore.Slot())
	pin := certStore.Pin()
	clientCred := certificate.TestCert1().ClientCred
	id, err := certStore.InstallCertKeyPair(ctx, clientCred.PrivateKey, clientCred.Cert)
	return CertVals{id, slot, pin}, err
}

func (f *vpnFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	var err error
	runner := hwsec.NewCmdRunner()
	f.certStore, err = netcertstore.CreateStore(ctx, runner)
	if err != nil {
		s.Fatal("Failed to create cert store: ", err)
	}

	certVals, err := installUserCert(ctx, f.certStore)
	if err != nil {
		s.Fatal("Failed to install cert: ", err)
	}

	if f.useCr {
		// Install CA cert to TPM. Since CA certs are stored as raw strings in
		// shill's profile, this is only required when Chrome is involved.
		if _, err := f.certStore.InstallCertKeyPair(ctx, "", certificate.TestCert1().CACred.Cert); err != nil {
			s.Fatal("Failed to install CA cert: ", err)
		}

		cred := chrome.Creds{User: netcertstore.TestUsername, Pass: netcertstore.TestPassword}
		f.cr, err = chrome.New(
			ctx,
			chrome.KeepState(),     // to avoid resetings TPM
			chrome.FakeLogin(cred), // to use the same user as certs are installed for
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}

	return FixtureEnv{f.cr, certVals}
}

func (f *vpnFixture) Reset(ctx context.Context) error {
	// When there is a failure and no Chrome, we only need to reset shill.
	if !f.useCr && f.hasError {
		f.hasError = false
		testing.ContextLog(ctx, "Test failed, reseting shill")
		if err := resetShillWithLockingHook(ctx); err != nil {
			return errors.Wrap(err, "failed to reset shill")
		}
		return nil
	}

	if !f.useCr {
		return nil
	}

	// We need to reset shill when the test failed, and thus it will invalidate
	// the shill profile known to Chrome. Since there seems to be no reliable way
	// to check that Chrome gets new profile, let's do a full restart of this
	// fixture.
	if f.hasError {
		f.hasError = false
		return errors.New("last test failed, triggering a full reset")
	}

	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset existing Chrome session")
	}
	return nil
}

func (f *vpnFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *vpnFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	f.hasError = s.HasError()
}

func (f *vpnFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.useCr {
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		f.cr = nil
	}

	if err := f.certStore.Cleanup(ctx); err != nil {
		s.Error("Failed to clean up cert store: ", err)
	}

	// Restart ui so that cryptohome unmounts all user mounts before shill is
	// restarted so that shill does not keep the mounts open perpetually.
	// TODO(b/205726835): Remove once the mount propagation for shill is fixed.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Error("Failed to restart ui: ", err)
	}

	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Error("Failed to reset shill in TearDown: ", err)
	}
}
