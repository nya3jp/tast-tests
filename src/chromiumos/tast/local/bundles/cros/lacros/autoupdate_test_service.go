// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"

	"google.golang.org/grpc"

	"chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			lacros.RegisterAutoupdateTestServiceServer(srv, &AutoupdateTestService{s: s})
		},
	})
}

// AutoupdateTestService implements tast.cros.lacros.AutoupdateTestService.
type AutoupdateTestService struct { // NOLINT
	s *testing.ServiceState
}

// VerifyLacrosVersion checks if the expected version of Lacros is loaded
// successfully without crash given the browser contexts.
func (auts *AutoupdateTestService) VerifyLacrosVersion(ctx context.Context, req *lacros.VerifyLacrosVersionRequest) (*lacros.VerifyLacrosVersionResponse, error) {
	// TODO: Implement VerifyLacrosVersion
	testing.ContextLog(ctx, "VerifyLacrosVersion, req=", req)
	res := &lacros.VerifyLacrosVersionResponse{
		Result: &lacros.TestResult{
			Status:        lacros.TestResult_PASSED,
			StatusDetails: "Okay",
		},
	}
	return res, nil
}

// GetBrowserVersion returns version info of the given browser type.
// If multiple Lacros browsers are provisioned in the stateful partition,
// all the versions will be returned.
func (auts *AutoupdateTestService) GetBrowserVersion(ctx context.Context, req *lacros.GetBrowserVersionRequest) (*lacros.GetBrowserVersionResponse, error) {
	// TODO: Implement GetBrowserVersion
	return &lacros.GetBrowserVersionResponse{
		Versions: []string{"99.9.9.9"},
	}, nil
}
