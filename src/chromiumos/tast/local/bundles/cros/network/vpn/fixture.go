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
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const certOpTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vpnShillReset",
		Desc: "A fixture that 1) install certificates which are required by some VPN services, and 2) resets shill to a default state when this fixture starts and ends, and after a test if it failed",
		Contacts: []string{
			"jiejiang@google.com",        // fixture maintainer
			"cros-networking@google.com", // platform networking team
		},
		SetUpTimeout:    shill.ResetShillTimeout + certOpTimeout + 5*time.Second,
		PostTestTimeout: shill.ResetShillTimeout + 5*time.Second,
		TearDownTimeout: shill.ResetShillTimeout + certOpTimeout + 5*time.Second,
		Impl:            &vpnFixture{},
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

type vpnFixture struct {
	certStore *netcertstore.Store
}

// CertVals contains the required values to setup a cert-based VPN service.
type CertVals struct {
	id   string
	slot string
	pin  string
}

func installCert(ctx context.Context, certStore *netcertstore.Store) (CertVals, error) {
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

	certVals, err := installCert(ctx, f.certStore)
	if err != nil {
		s.Fatal("Failed to install cert: ", err)
	}

	return certVals
}

func (f *vpnFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *vpnFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *vpnFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if !s.HasError() {
		return
	}

	// Resets shill when the test failed. We assume that a successful test run
	// will not leave shill in a state which can affect the following tests.
	testing.ContextLog(ctx, "Test failed, reseting shill")
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Error("Failed to reset shill in PostTest: ", err)
	}
}

func (f *vpnFixture) TearDown(ctx context.Context, s *testing.FixtState) {
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
