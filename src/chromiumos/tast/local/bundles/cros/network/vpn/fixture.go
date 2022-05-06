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

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vpnShillReset",
		Desc: "A fixture that ensures shill is in a default state when the test starts and resets shill after the test if it failed",
		Contacts: []string{
			"jiejiang@google.com",        // fixture maintainer
			"cros-networking@google.com", // platform networking team
		},
		SetUpTimeout:    shill.ResetShillTimeout + 5*time.Second,
		PostTestTimeout: shill.ResetShillTimeout + 5*time.Second,
		TearDownTimeout: shill.ResetShillTimeout + 5*time.Second,
		Impl:            &vpnFixture{},
	})
}

func resetShillWithLockingHook(ctx context.Context) error {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us. Lock the hook
	// before shill restarted.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	if errs := shill.ResetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			testing.ContextLog(ctx, "ResetShill error: ", err)
		}
		return errs[0]
	}

	return nil
}

type vpnFixture struct {
	certInfo *CertStoreInfo
}

func (f *vpnFixture) prepareCertStore(ctx context.Context) error {
	runner := hwsec.NewCmdRunner()
	certStore, err := netcertstore.CreateStore(ctx, runner)
	if err != nil {
		return errors.Wrap(err, "failed to create cert store")
	}

	certSlot := fmt.Sprintf("%d", certStore.Slot())
	certPin := certStore.Pin()
	clientCred := certificate.TestCert1().ClientCred
	certID, err := certStore.InstallCertKeyPair(ctx, clientCred.PrivateKey, clientCred.Cert)
	if err != nil {
		return errors.Wrap(err, "failed to insert cert key pair into cert store")
	}

	f.certInfo = &CertStoreInfo{certStore, certID, certSlot, certPin}
	return nil
}

func (f *vpnFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	if err := f.prepareCertStore(ctx); err != nil {
		s.Fatal("Failed to prepare cert store: ", err)
	}

	return f.certInfo
}

func (f *vpnFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *vpnFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *vpnFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if !s.HasError() {
		return
	}

	testing.ContextLog(ctx, "Test failed, reseting shill")
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Error("Failed to reset shill in PostTest: ", err)
	}
}

func (f *vpnFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.certInfo.certStore.Cleanup(ctx); err != nil {
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
