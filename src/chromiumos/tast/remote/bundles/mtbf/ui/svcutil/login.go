// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package svcutil

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/svcutil"
	"chromiumos/tast/testing"
)

// Login for GRPC Login
func Login(ctx context.Context, s *testing.State) {
	// New GRPC connection
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer cl.Close(ctx)

	// New GRPC Client
	cr := svcutil.NewCommServiceClient(cl.Conn)

	if _, err := cr.Login(ctx, &empty.Empty{}); err != nil {
		s.Fatal(mtbferrors.NewGRPCErr(err))
	}
}
