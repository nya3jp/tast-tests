// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/network/netconfig"
	pb "chromiumos/tast/services/cros/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	var osSettingsService Service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			osSettingsService = Service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterOsSettingsServiceServer(srv, &osSettingsService)
		},
	})
}

// Service implements tast.cros.chrome.uiauto.ossettings.OsSettingsService
type Service struct {
	sharedObject *common.SharedObjectsForService
}

func computeNetworkConfigNetworkType(networkType pb.OpenNetworkDetailPageRequest_NetworkType) (netconfig.NetworkType, error) {
	if networkType == pb.OpenNetworkDetailPageRequest_CELLULAR {
		return netconfig.Cellular, nil
	}
	if networkType == pb.OpenNetworkDetailPageRequest_WIFI {
		return netconfig.WiFi, nil
	}
	return 0, errors.New("Network type must be Cellular or WiFi")
}

// OpenNetworkDetailPage will open the OS Settings application and navigate
// to the detail page for the specified network.
func (s *Service) OpenNetworkDetailPage(ctx context.Context, req *pb.OpenNetworkDetailPageRequest) (*empty.Empty, error) {
	cr := s.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome has not been started")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create test API connection")
	}
	networkType, err := computeNetworkConfigNetworkType(req.NetworkType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to determine network type")
	}
	if _, err = OpenNetworkDetailPage(ctx, tconn, cr, req.NetworkName, networkType); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to navigate to network detail page")
	}
	return &empty.Empty{}, nil
}

// Close will close the open OS Settings application.
func (s *Service) Close(ctx context.Context, e *empty.Empty) (*empty.Empty, error) {
	cr := s.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome has not been started")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create test API connection")
	}
	if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to close OS Settings")
	}
	return &empty.Empty{}, nil
}
