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
	nws "chromiumos/tast/local/bundles/cros/autoupdate/rollbacknetworks"
	"chromiumos/tast/local/chrome"
	nc "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/lsbrelease"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

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

// SetUpNetworks sets up a series of network configuration on the device that
// are supported by rollback.
// The device needs to be in a state so that chrome://network may be opened.
func (r *RollbackService) SetUpNetworks(ctx context.Context, request *aupb.SetUpNetworksRequest) (*aupb.SetUpNetworksResponse, error) {
	testing.ContextLog(ctx, "setting up networks supported by rollback")
	// Open chrome and create a connection to the network configuration api.
	// This is needed to set up each network without having to create a connection
	// each time.
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

	// Set up the supported networks.
	var networks []*aupb.NetworkInformation
	for _, nw := range nws.SupportedNetworks {
		nwInfo, err := setUpNetwork(ctx, api, nw.Config)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to set up %s network", nw.Type)
		}
		networks = append(networks, nwInfo)
	}

	networksResponse := &aupb.SetUpNetworksResponse{
		Networks: networks,
	}

	testing.ContextLogf(ctx, "Networks set: %s ", string(networksResponse.String()))
	return networksResponse, nil
}

// setUpNetwork sets up a network configuration on the device.
func setUpNetwork(ctx context.Context, api *nc.CrosNetworkConfig, properties nc.ConfigProperties) (*aupb.NetworkInformation, error) {
	guid, err := api.ConfigureNetwork(ctx, properties, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure network")
	}
	networkResponse := &aupb.NetworkInformation{
		Guid: guid,
	}
	return networkResponse, nil
}

// VerifyRollback checks that the device is on the enrollment screen, then logs
// in as a normal user and verifies the previously set-up networks exists.
// VerifyRollbackRequest needs to contain the unchanged NetworkInformation from
// SetUpNetworksResponse.
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

	testing.ContextLogf(ctx, "Verify preservation of networks set: %s ", request.Networks)
	for idx, networkInfo := range request.Networks {
		// Obtain properties of network set.
		guid := networkInfo.Guid
		managedProperties, err := api.GetManagedProperties(ctx, guid)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get managed properties for guid %s", guid)
		}

		// Retrieve corresponding supported network configuration.
		// It is assumed that the guid are received in the same order they were set,
		// so we use the idx to identify which is the corresponding network.
		nwID := nws.ConfigID(idx)
		preservedNw, err := nws.VerifyNetwork(ctx, nwID, managedProperties)
		if err != nil {
			return nil, errors.Wrap(err, "failed to verify network")
		}

		if !preservedNw {
			response.Successful = false
			response.VerificationDetails += nws.SupportedNetworks[nwID].Type + " network was not preserved;"
		}
	}

	return response, nil
}

// SetUpPskNetwork is deprecated. Use SetUpNetworks instead.
func (r *RollbackService) SetUpPskNetwork(ctx context.Context, req *empty.Empty) (*aupb.SetUpPskResponse, error) {
	return nil, errors.New("use of deprecated SetUpPskNetwork; SetUpNetworks should be used instead")
}
