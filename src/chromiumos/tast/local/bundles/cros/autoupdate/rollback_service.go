// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	nc "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/lsbrelease"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

var psk = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: nc.WiFiConfigProperties{
			Passphrase: "pass,pass,123",
			Ssid:       "MyHomeWiFi",
			Security:   nc.WpaPsk,
			HiddenSsid: nc.Automatic}}}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			aupb.RegisterRollbackServiceServer(srv, &RollbackService{s: s})
		},
	})
}

// RollbackService implements tast.cros.autoupdate.RollbackService.
type RollbackService struct {
	s *testing.ServiceState
}

// SetUpPskNetwork sets up a simple psk network configuration on the device.
// The device needs to be in a state so that chrome://network may be opened.
func (r *RollbackService) SetUpPskNetwork(ctx context.Context, req *empty.Empty) (*aupb.SetUpPskResponse, error) {
	cr, err := chrome.New(ctx, chrome.KeepEnrollment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	api, err := nc.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cros network config api")
	}
	defer api.Close(ctx)

	guid, err := api.ConfigureNetwork(ctx, psk, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure psk network")
	}
	return &aupb.SetUpPskResponse{Guid: guid}, nil
}

// VerifyRollback checks that the device is on the enrollment screen, then logs
// in as a normal user and verifies the previously set-up network exists.
func (r *RollbackService) VerifyRollback(ctx context.Context, request *aupb.VerifyRollbackRequest) (*aupb.VerifyRollbackResponse, error) {
	cr, err := chrome.New(ctx, chrome.DeferLogin())
	if err != nil {
		return nil, errors.Wrap(err, "failed to restart Chrome for testing after rollback")
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

	// The following checks are expected to fail on any milestone <100 because
	// Chrome was not ready for rollback tests yet. We skip them and consider the
	// verification is successful, but we inform that a full verification was not
	// possible.
	lsbContent, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /etc/lsb-release")
	}

	milestoneVal, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert milestone %s to integer", lsbContent[lsbrelease.Milestone])
	}

	response := &aupb.VerifyRollbackResponse{
		Successful:          true,
		VerificationDetails: "",
	}

	if milestoneVal < 100 {
		response.Successful = true
		response.VerificationDetails = "Image does not support a full rollback verification"
		return response, nil
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to login as normal user after rollback")
	}

	api, err := nc.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cros network config api")
	}
	defer api.Close(ctx)

	managedProperties, err := api.GetManagedProperties(ctx, request.Guid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get managed properties for guid %s", request.Guid)
	}

	// Passphrase is not passed via cros_network_config, instead mojo passes a constant value if a password is configured. Only check for non-empty.
	if managedProperties.TypeProperties.Wifi.Security !=
		psk.TypeConfig.Wifi.Security ||
		managedProperties.TypeProperties.Wifi.Ssid.ActiveValue !=
			psk.TypeConfig.Wifi.Ssid ||
		managedProperties.TypeProperties.Wifi.Passphrase.ActiveValue == "" {
		response.Successful = false
		response.VerificationDetails = "PSK network was not preserved"
	}

	return response, nil
}
