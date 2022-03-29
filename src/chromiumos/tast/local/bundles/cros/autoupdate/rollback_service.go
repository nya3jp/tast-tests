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

const PskRef = "psk"

// Simple PSK network configuration
var psk = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: nc.WiFiConfigProperties{
			Passphrase: "pass,pass,123",
			Ssid:       "MyHomeWiFi",
			Security:   nc.WpaPsk,
			HiddenSsid: nc.Automatic}}}

const PeapWifiRef = "peapWifi"

// PEAP wifi configuration without certificates
var peapWifi = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: nc.WiFiConfigProperties{
			Eap: &nc.EAPConfigProperties{
				AnonymousIdentity:   "anonymous_identity",
				Identity:            "userIdentity",
				Inner:               "Automatic",
				Outer:               "PEAP",
				Password:            "testPass",
				SaveCredentials:     true,
				ClientCertType:      "None",
				DomainSuffixMatch:   []string{},
				SubjectAltNameMatch: []nc.SubjectAltName{},
				UseSystemCAs:        false,
			},
			Ssid:       "wifiTestPEAP",
			Security:   nc.WpaEap,
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

	// Set up the networks.
	// Test a simple PSK network configuration and PEAP without certificates.
	// TODO(b/227562233): Test all the type of networks that are supported and
	// preserved during rollback.
	var networks []*aupb.NetworkInformation
	pskNetwork, err := SetUpNetwork(ctx, api, psk, PskRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up PSK network")
	}
	networks = append(networks, pskNetwork)

	peapWifiNetwork, err := SetUpNetwork(ctx, api, peapWifi, PeapWifiRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wifi PEAP network")
	}
	networks = append(networks, peapWifiNetwork)

	networksResponse := &aupb.SetUpNetworksResponse{
		Networks: networks,
	}

	testing.ContextLogf(ctx, "Networks set: %s ", string(networksResponse.String()))
	return networksResponse, nil
}

// SetUpNetwork sets up a network configuration on the device.
func SetUpNetwork(ctx context.Context, api *nc.CrosNetworkConfig, properties nc.ConfigProperties, ref string) (*aupb.NetworkInformation, error) {
	guid, err := api.ConfigureNetwork(ctx, properties, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure network")
	}
	networkResponse := &aupb.NetworkInformation{
		Guid:      guid,
		Reference: ref,
	}
	return networkResponse, nil
}

// VerifyRollback checks that the device is on the enrollment screen, then logs
// in as a normal user and verifies the previously set-up networks exists.
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
	for _, networkInfo := range request.Networks {
		guid := networkInfo.Guid
		reference := networkInfo.Reference
		managedProperties, err := api.GetManagedProperties(ctx, guid)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get managed properties for guid %s", guid)
		}

		// Passphrase and Password are not passed via cros_network_config, instead
		// mojo passes a constant value if a password is configured. Only check for
		// non-empty.
		switch reference {
		case PskRef:
			pskSet := managedProperties.TypeProperties.Wifi
			pskExp := psk.TypeConfig.Wifi
			if pskSet.Security != pskExp.Security ||
				pskSet.Ssid.ActiveValue != pskExp.Ssid ||
				pskSet.Passphrase.ActiveValue == "" {
				response.Successful = false
				response.VerificationDetails += "PSK network was not preserved;"
				// Log details about existing and expected configuration for debugging.
				testing.ContextLogf(ctx, "Set (managedProperties.Wifi): %+v", pskSet)
				testing.ContextLogf(ctx, "Expected (psk.Wifi): %+v", pskExp)
			}
		case PeapWifiRef:
			// ClientCertType, independently of the value set, is PKCS11Id. Only check
			// for non-empty.
			// TODO(crisguerrero): Add check of Eap.Inner when b/227605505 is fixed.
			peapWifiSet := managedProperties.TypeProperties.Wifi
			peapWifiExp := peapWifi.TypeConfig.Wifi
			if peapWifiSet.Security != peapWifiExp.Security ||
				peapWifiSet.Ssid.ActiveValue != peapWifiExp.Ssid ||
				peapWifiSet.Eap.AnonymousIdentity.ActiveValue != peapWifiExp.Eap.AnonymousIdentity ||
				peapWifiSet.Eap.Identity.ActiveValue != peapWifiExp.Eap.Identity ||
				peapWifiSet.Eap.Outer.ActiveValue != peapWifiExp.Eap.Outer ||
				peapWifiSet.Eap.Password.ActiveValue == "" ||
				peapWifiSet.Eap.SaveCredentials.ActiveValue != peapWifiExp.Eap.SaveCredentials ||
				peapWifiSet.Eap.ClientCertType.ActiveValue == "" ||
				peapWifiSet.Eap.UseSystemCAs.ActiveValue != peapWifiExp.Eap.UseSystemCAs {
				response.Successful = false
				response.VerificationDetails += "PEAP wifi network was not preserved;"
				// Log details about existing and expected configuration for debugging.
				testing.ContextLogf(ctx, "Set (managedProperties.Wifi): %+v", peapWifiSet)
				testing.ContextLogf(ctx, "Expected (peapWifi.Wifi): %+v", peapWifiExp)
				testing.ContextLogf(ctx, "Set (managedProperties.Wifi.Eap): %+v", peapWifiSet.Eap)
				testing.ContextLogf(ctx, "Expected (peapWifi.Wifi.Eap): %+v", peapWifiExp.Eap)
			}
		default:
			return nil, errors.Errorf("failed to find network properties for %s", reference)
		}
	}

	return response, nil
}

// SetUpPskNetwork is deprecated. Use SetUpNetworks instead.
func (r *RollbackService) SetUpPskNetwork(ctx context.Context, req *empty.Empty) (*aupb.SetUpPskResponse, error) {
	return nil, errors.New("use of deprecated SetUpPskNetwork; SetUpNetworks should be used instead")
}
