// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/tape"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	ts "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.CleanOwnership,
		Desc: "Fixture cleaning enrollment related state. Use when your local test performs REAL enrollment and proper clean up is necessary. DUT reboots before and after all tests using the fixture",
		Contacts: []string{
			"kamilszarek@google.com",
			"chromeos-commercial-remote-management@google.com"},
		Impl:            &cleanOwner{},
		SetUpTimeout:    3 * time.Minute,
		TearDownTimeout: 2 * time.Minute,
		ResetTimeout:    2 * time.Minute,
		PostTestTimeout: 2 * time.Minute,
		ServiceDeps:     []string{"tast.cros.policy.PolicyService", "tast.cros.tape.Service"},
		Vars: []string{
			"tape.service_account_key",
			"tape.managedchrome_id",
		},
	})
}

type cleanOwner struct {
	// dut holds the reference to DUT that is needed in Reset() function as
	// there is no access to the DUT.
	dut               *dut.DUT
	customerID        string
	serviceAccountKey string
}

func (co *cleanOwner) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
	co.dut = s.DUT()

	co.customerID = s.RequiredVar("tape.managedchrome_id")
	co.serviceAccountKey = s.RequiredVar("tape.service_account_key")

	return nil
}

func (co *cleanOwner) TearDown(ctx context.Context, s *testing.FixtState) {
	// After the last test we want to clean ownership.
	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
}

func (co *cleanOwner) Reset(ctx context.Context) error {
	// Between each tests we want to clean ownership. The last test won't have
	// this executed hence we have to do it in TearDown().
	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, co.dut); err != nil {
		return errors.Wrap(err, "failed to reset TPM")
	}
	return nil
}
func (co *cleanOwner) PreTest(ctx context.Context, s *testing.FixtTestState) {
}
func (co *cleanOwner) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// After each test we want to deprovision device if it was enrolled.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	tapeService := ts.NewServiceClient(cl.Conn)

	// Get the device id of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: co.customerID})
	if err != nil {
		s.Fatal("Failed to get the deviceID: ", err)
	}

	tapeClient, err := tape.NewClient(ctx, []byte(co.serviceAccountKey))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}
	if err = tapeClient.Deprovision(ctx, res.DeviceID, co.customerID); err != nil {
		s.Fatalf("Failed to deprovision device %s: %v", res.DeviceID, err)
	}
}
