// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"time"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

const (
	// RemoteComRobloxClientLeasedAccountFixture is a remote fixture which is used for Roblox testing.
	RemoteComRobloxClientLeasedAccountFixture = "tapeRemoteComRobloxClientLeasedAccountFixture"

	// RobloxTestPoolID is the name of the pool id associated with Roblox accounts.
	RobloxTestPoolID = "com.roblox.client"

	// RobloxLeaseTimeForAllTestsInSeconds is the amount of time needed for all Roblox tests.
	RobloxLeaseTimeForAllTestsInSeconds = 60 * 60 * 1 // 1 hour.

	// remoteFixtureTimeouts stores the timeouts for the Remote fixture functionality.
	remoteFixtureTimeout = 5 * time.Minute
)

// LeasedAccountFileData holds the data to identify the leased account. This information is stored on the local DUT.
type LeasedAccountFileData struct {
	Username string `json:"username"`
}

type remoteLeasedAccountFixture struct {
	poolID           string
	timeoutInSeconds int32
	genericAccount   GenericAccount
}

// NewRemoteLeasedAccountFixture returns the fixture implementation for a remote fixture which is associated with a pool.
func NewRemoteLeasedAccountFixture(poolID string, timeoutInSeconds int32) testing.FixtureImpl {
	return &remoteLeasedAccountFixture{
		poolID:           poolID,
		timeoutInSeconds: timeoutInSeconds,
	}
}
func (t *remoteLeasedAccountFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Get a generic account for testing with the provided pool id and timeout.
	client, err := NewTapeClient(ctx)
	if err != nil {
		s.Fatal("Failed to setup TAPE client: ", err)
	}

	params := RequestGenericAccountParams{
		TimeoutInSeconds: t.timeoutInSeconds,
		PoolID:           &t.poolID,
	}

	gar, err := RequestGenericAccount(ctx, params, client)
	if err != nil {
		s.Fatal("Failed to request generic account: ", err)
	}
	t.genericAccount = *gar

	// Write the content to the DUT.
	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial: ", err)
	}
	defer c.Close(ctx)

	tsc := tape.NewTapeServiceClient(c.Conn)
	if _, err := tsc.SaveGenericAccountInfoToFile(ctx, &tape.SaveGenericAccountInfoToFileRequest{
		Path:     LocalDUTAccountFileLocation(t.poolID),
		Username: gar.Username,
	}); err != nil {
		s.Fatal("Failed to save generic account to DUT: ", err)
	}

	return nil
}
func (t *remoteLeasedAccountFixture) Reset(ctx context.Context) error                        { return nil }
func (t *remoteLeasedAccountFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (t *remoteLeasedAccountFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
func (t *remoteLeasedAccountFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Release the generic account.
	client, err := NewTapeClient(ctx)
	if err != nil {
		s.Fatal("Failed to setup TAPE client: ", err)
	}

	if err := ReleaseGenericAccount(ctx, &t.genericAccount, client); err != nil {
		s.Fatal("Failed to request generic account: ", err)
	}

	// Remove the data from the DUT.
	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial: ", err)
	}
	defer c.Close(ctx)

	tsc := tape.NewTapeServiceClient(c.Conn)
	if _, err := tsc.RemoveGenericAccountInfo(ctx, &tape.RemoveGenericAccountInfoRequest{
		Path: LocalDUTAccountFileLocation(t.poolID),
	}); err != nil {
		s.Fatal("Failed to remove generic account information from DUT: ", err)
	}
}

// RemoteFixtures is a convenience method which holds all remote fixtures for
// TAPE. They will all be automatically registered. Note that no two remote fixtures
// should define the same PoolID. This is a requirement of the current implementation.
func RemoteFixtures() []*testing.Fixture {
	return []*testing.Fixture{
		&testing.Fixture{
			Name:            RemoteComRobloxClientLeasedAccountFixture,
			Desc:            "Remote fixture which stores a Roblox account on a DUT. The account is unique to the DUT and is guaranteed to not be in use on other DUTs",
			Contacts:        []string{"davidwelling@google.com", "arc-engprod@google.com"},
			Impl:            NewRemoteLeasedAccountFixture(RobloxTestPoolID, RobloxLeaseTimeForAllTestsInSeconds),
			SetUpTimeout:    remoteFixtureTimeout,
			TearDownTimeout: remoteFixtureTimeout,
			ServiceDeps:     []string{"tast.cros.tape.TapeService"},
		},
	}
}
