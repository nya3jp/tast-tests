// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "enrolled",
		Desc:            "Fixture profviding enrollment",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-stability@google.com"},
		Impl:            &enrolledFixt{},
		SetUpTimeout:    5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ResetTimeout:    15 * time.Second,
		ServiceDeps:     []string{"tast.cros.policy.PolicyService"},
	})
}

type enrolledFixt struct {
	fdmsDir string
}

func (e *enrolledFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	// TODO(crbug.com/1187473): use a temporary directory.
	e.fdmsDir = fakedms.EnrollmentFakeDMSDir

	if _, err := pc.CreateFakeDMSDir(ctx, &ps.CreateFakeDMSDirRequest{
		Path: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to create temporary directory for FakeDMS: ", err)
	}

	pJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		FakedmsDir: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to enroll using Chrome: ", err)
	}

	if _, err := pc.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to stop Chrome: ", err)
	}

	return nil
}

func (e *enrolledFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.RemoveFakeDMSDir(ctx, &ps.RemoveFakeDMSDirRequest{
		Path: e.fdmsDir,
	}); err != nil {
		s.Fatal("Failed to remove temporary directory for FakeDMS: ", err)
	}
}

func (*enrolledFixt) Reset(ctx context.Context) error                        { return nil }
func (*enrolledFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*enrolledFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
