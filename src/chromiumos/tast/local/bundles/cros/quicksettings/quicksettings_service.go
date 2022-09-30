// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var quicksettingsService Service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			quickSettingsService = Service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterAutomationServiceServer(srv, &quickSettingsService)
		},
		GuaranteeCompatability: true,
	})
}

// Service implements tast.cros.ui.Service
type Service struct {
	sharedObject *common.SharedObjectsForService
}

// NavigateToNetworkDetailedView will navigate to the detailed Network view
// within the Quick Settings. This is safe to call even when the Quick Settings
// are already open.
func (s *Service) NavigateToNetworkDetailedView(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	cr := s.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome has not been started")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create test API connection")
	}
	return &empty.Empty{}, quicksettings.NavigateToNetworkDetailedView(ctx, tconn, true)
}
