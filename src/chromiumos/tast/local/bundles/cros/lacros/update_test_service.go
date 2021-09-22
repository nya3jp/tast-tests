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
			lacros.RegisterUpdateTestServiceServer(srv, &UpdateTestService{s: s})
		},
	})
}

// UpdateTestService implements tast.cros.lacros.UpdateTestService.
type UpdateTestService struct {
	s *testing.ServiceState
}

// VerifyUpdate checks if the expected version of Lacros is loaded successfully without crash given the browsers provisioned.
func (uts *UpdateTestService) VerifyUpdate(ctx context.Context, req *lacros.VerifyUpdateRequest) (*lacros.VerifyUpdateResponse, error) {
	// TODO: Implement VerifyUpdate
	return &lacros.VerifyUpdateResponse{
		Result: &lacros.TestResult{
			Status:        lacros.TestResult_PASSED,
			StatusDetails: "Okay",
		},
	}, nil
}

// ClearUpdate removes provisioned Lacros in the install path.
func (uts *UpdateTestService) ClearUpdate(ctx context.Context, req *lacros.ClearUpdateRequest) (*lacros.ClearUpdateResponse, error) {
	// TODO: Implement ClearUpdate
	return &lacros.ClearUpdateResponse{}, nil
}

// GetBrowserVersion returns version info of the given browser type.
// If multiple Lacros browsers are provisioned in the stateful partition, all the versions will be returned.
func (uts *UpdateTestService) GetBrowserVersion(ctx context.Context, req *lacros.GetBrowserVersionRequest) (*lacros.GetBrowserVersionResponse, error) {
	// TODO: Implement GetBrowserVersion
	return &lacros.GetBrowserVersionResponse{
		Versions: []string{"99.9.9.9"},
	}, nil
}
