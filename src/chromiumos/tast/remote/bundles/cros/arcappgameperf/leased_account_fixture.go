// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	tape2 "chromiumos/tast/common/tape"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

const (
	RobloxTestPool                      = "test_pool"
	AllRobloxTestsTotalTimeoutInSeconds = 3600
)

func init() {
	// TODO(b/222311973): Right now, every test would have to create a base fixture here and set the timeout to be long enough to support all tests that use the fixture.
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.RobloxLeasedAccountFixture,
		Desc:            "Remote fixture which stores a Roblox account on a DUT. The account is unique to the DUT and is guaranteed to not be in use on other DUTs",
		Contacts:        []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Impl:            NewLeasedAccountFixture(RobloxTestPool, AllRobloxTestsTotalTimeoutInSeconds),
		SetUpTimeout:    10 * time.Minute,
		TearDownTimeout: 10 * time.Minute,
		ResetTimeout:    10 * time.Minute,
		ServiceDeps:     []string{"tast.cros.tape.TapeService"},
	})
}

type leasedAccountFixture struct {
	poolId           string
	timeoutInSeconds int32
	genericAccount   *tape2.GenericAccount
}

func NewLeasedAccountFixture(poolId string, timeoutInSeconds int32) testing.FixtureImpl {
	return &leasedAccountFixture{
		poolId:           poolId,
		timeoutInSeconds: timeoutInSeconds,
	}
}

func (t *leasedAccountFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Get a generic account for testing with the provided pool id and timeout.
	client, err := tape2.NewTapeClient(ctx)
	if err != nil {
		s.Fatal("Failed to setup TAPE client: ", err)
	}

	params := tape2.RequestGenericAccountParams{
		TimeoutInSeconds: t.timeoutInSeconds,
		PoolID:           &t.poolId,
	}

	gar, err := tape2.RequestGenericAccount(ctx, params, client)
	if err != nil {
		s.Fatal("Failed to request generic account: ", err)
	}
	t.genericAccount = gar

	// Save the data down to the DUT for use in its fixture.
	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial: ", err)
	}
	defer c.Close(ctx)

	// TODO (b/222311973): Remote fixtures can't return data currently (b/187957164) otherwise a write to /tmp/account.json wouldn't be needed.
	tsc := tape.NewTapeServiceClient(c.Conn)
	if _, err := tsc.SaveGenericAccountInfoToFile(ctx, &tape.SaveGenericAccountInfoToFileRequest{
		Path:     "/tmp/account.json",
		Username: gar.Username,
		Password: gar.Password,
	}); err != nil {
		s.Fatal("Failed to save generic account to DUT: ", err)
	}

	return nil
}
func (t *leasedAccountFixture) Reset(ctx context.Context) error { return nil }

func (t *leasedAccountFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO (b/222311973): It would be nice if the call for requesting an account could be made here.
}

func (t *leasedAccountFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO (b/222311973): It would be nice if the call for releasing an account could be made here.
}

func (t *leasedAccountFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Release the generic account.
	client, err := tape2.NewTapeClient(ctx)
	if err != nil {
		s.Fatal("Failed to setup TAPE client: ", err)
	}

	if err := tape2.ReleaseGenericAccount(ctx, t.genericAccount, client); err != nil {
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
		Path: "/tmp/account.json",
	}); err != nil {
		s.Fatal("Failed to remove generic account information from DUT: ", err)
	}
}
