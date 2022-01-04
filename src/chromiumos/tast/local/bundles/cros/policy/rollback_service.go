// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	network "chromiumos/tast/local/network/cros_network_config"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

var psk = network.ConfigProperties{
	TypeConfig: network.NetworkTypeConfigProperties{
		Wifi: network.WiFiConfigProperties{
			Passphrase: "pass,pass,123",
			Ssid:       "MyHomeWiFi",
			Security:   network.WpaPsk,
			HiddenSsid: network.Automatic}}}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterRollbackServiceServer(srv, &RollbackService{s: s})
		},
	})
}

// RollbackService implements tast.cros.policy.RollbackService.
type RollbackService struct {
	s *testing.ServiceState
}

func (r *RollbackService) SetUpPskNetwork(ctx context.Context, req *empty.Empty) (*ppb.Guid, error) {
	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.TryReuseSession())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	api, err := network.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cros network config api")
	}
	defer api.Close(ctx)

	guid, err := api.ConfigureNetwork(ctx, psk, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure psk network")
	}
	return &ppb.Guid{Guid: guid}, nil
}

func (r *RollbackService) VerifyRollback(ctx context.Context, guid *ppb.Guid) (*ppb.RollbackSuccessfulResponse, error) {
	response := &ppb.RollbackSuccessfulResponse{
		Successful: true,
	}

	cr, err := chrome.New(ctx, chrome.DeferLogin())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.signInStep.isReadyForTesting()"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for enrollment screen")
	}

	cr.ContinueLogin(ctx)

	api, err := network.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cros network config api")
	}
	defer api.Close(ctx)

	managedProperties, err := api.GetManagedProperties(ctx, guid.Guid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get managed properties for guid %s", guid.Guid)
	}

	// Passphrase is not passed via cros_network_config, instead mojo passes a dummy value if a password is configured.
	if managedProperties.TypeProperties.Wifi.Security !=
		psk.TypeConfig.Wifi.Security ||
		managedProperties.TypeProperties.Wifi.Ssid.ActiveValue !=
			psk.TypeConfig.Wifi.Ssid ||
		managedProperties.TypeProperties.Wifi.Passphrase.ActiveValue == "" {
		response.Successful = false
	}

	return response, nil
}
