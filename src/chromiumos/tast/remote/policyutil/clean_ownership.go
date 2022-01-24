// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"net/http"
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
	tc                *http.Client
	customerID        string
	serviceAccountKey string
	cl                *rpc.Client
}

func (co *cleanOwner) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
	co.dut = s.DUT()

	tapeClient, err := tape.NewTapeClient(ctx, tape.WithCredsJSON([]byte(s.RequiredVar("tape.service_account_key"))))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}

	co.tc = tapeClient
	co.customerID = s.RequiredVar("tape.managedchrome_id")
	co.serviceAccountKey = s.RequiredVar("tape.service_account_key")

	co.cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	return nil
}

func (co *cleanOwner) TearDown(ctx context.Context, s *testing.FixtState) {
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

	var request tape.DeprovisionRequest
	request.DeviceID = res.DeviceID
	request.CustomerID = co.customerID
	tapeClient, err := tape.NewTapeClient(ctx, tape.WithCredsJSON([]byte(co.serviceAccountKey)))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}
	if err = tape.Deprovision(ctx, request, tapeClient); err != nil {
		s.Fatalf("Failed to deprovision device %s: %v", request.DeviceID, err)
	}

	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
}

func (co *cleanOwner) Reset(ctx context.Context) error {
	tapeService := ts.NewServiceClient(co.cl.Conn)

	// Get the device id of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: co.customerID})
	if err != nil {
		return errors.Wrap(err, "failed to get the deviceID")
	}

	var request tape.DeprovisionRequest
	request.DeviceID = res.DeviceID
	request.CustomerID = co.customerID
	tapeClient, err := tape.NewTapeClient(ctx, tape.WithCredsJSON([]byte(co.serviceAccountKey)))
	if err != nil {
		return errors.Wrap(err, "failed to create tape client")
	}
	if err = tape.Deprovision(ctx, request, tapeClient); err != nil {
		return errors.Wrapf(err, "failed to deprovision device %s", request.DeviceID)
	}

	if err := EnsureTPMAndSystemStateAreResetRemote(ctx, co.dut); err != nil {
		return errors.Wrap(err, "failed to reset TPM")
	}
	return nil
}
func (*cleanOwner) PreTest(ctx context.Context, s *testing.FixtTestState) {
}
func (*cleanOwner) PostTest(ctx context.Context, s *testing.FixtTestState) {}
