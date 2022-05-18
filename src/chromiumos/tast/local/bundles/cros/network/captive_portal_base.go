// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CaptivePortalBase,
		Desc:     "Verifies that properties affected by Captive Portal are default when there is no Captive Portal active",
		Contacts: []string{"michaelrygiel@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func CaptivePortalBase(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)

	// Get Manager Properties
	managerProps, err := manager.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get the shill manager properties: ", err)
	}
	// Manager CheckPortalList
	managerCheckPortalList, err := managerProps.GetString(shillconst.ManagerPropertyCheckPortalList)
	if err != nil {
		s.Fatal("Failed to get managerCheckPortalList: ", err)
	}
	if managerCheckPortalList != shillconst.ManagerCheckPortalListNone {
		s.Fatal("ManagerCheckPortalList is not default: ", managerCheckPortalList)
	}

	// Get Service Properties
	props := map[string]interface{}{
		shillconst.ServicePropertyType: shillconst.TypeEthernet,
	}
	service, err := manager.FindMatchingService(ctx, props)
	if err != nil {
		s.Fatal("No Ethernet Service: ", err)
	}
	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		s.Fatal("No Service props: ", err)
	}

	// Service CheckPortal
	serviceCheckPortal, err := serviceProps.GetString(shillconst.ServicePropertyCheckPortal)
	if err != nil {
		s.Fatal("Failed to get serviceCheckPortal: ", err)
	}
	if serviceCheckPortal != shillconst.ServiceCheckPortalAuto {
		s.Fatal("ServiceCheckPortal is not auto: ", serviceCheckPortal)
	}

	// Service State
	serviceState, err := serviceProps.GetString(shillconst.ServicePropertyState)
	if err != nil {
		s.Fatal("Failed to get serviceState: ", err)
	}
	if serviceState != shillconst.ServiceStateOnline {
		s.Fatal("ServiceState is not online: ", serviceState)
	}

	// Service PortalDetectionFailedPhase
	servicePortalDetectionFailedPhase, err := serviceProps.GetString(shillconst.ServicePropertyPortalDetectionFailedPhase)
	if err != nil {
		s.Fatal("Failed to get servicePortalDetectionFailedPhase: ", err)
	}
	if servicePortalDetectionFailedPhase != shillconst.ServicePortalDetectionFailedPhaseNone {
		s.Fatal("ServicePortalDetectionFailedPhase is not empty: ", servicePortalDetectionFailedPhase)
	}

	// Service PortalDetectionFailedStatus
	servicePortalDetectionFailedStatus, err := serviceProps.GetString(shillconst.ServicePropertyPortalDetectionFailedStatus)
	if err != nil {
		s.Fatal("Failed to get servicePortalDetectionFailedStatus: ", err)
	}
	if servicePortalDetectionFailedStatus != shillconst.ServicePortalDetectionFailedStatusNone {
		s.Fatal("ServicePortalDetectionFailedStatus is not empty: ", servicePortalDetectionFailedStatus)
	}

	// Service PortalDetectionFailedStatusCode
	servicePortalDetectionFailedStatusCode, err := serviceProps.GetInt32(shillconst.ServicePropertyPortalDetectionFailedStatusCode)
	if err != nil {
		s.Fatal("Failed to get servicePortalDetectionFailedStatusCode: ", err)
	}
	if servicePortalDetectionFailedStatusCode != shillconst.ServicePortalDetectionFailedStatusCodeZero {
		s.Fatal("servicePortalDetectionFailedStatusCode is not 0: ", servicePortalDetectionFailedStatusCode)
	}

	// Service ProbeUrl
	serviceProbeURL, err := serviceProps.GetString(shillconst.ServicePropertyProbeURL)
	if err != nil {
		s.Fatal("Failed to get ServicePropertyProbeURL: ", err)
	}
	if serviceProbeURL != shillconst.ServiceProbeURLNone {
		s.Fatal("ServicePropertyProbeURL is not empty: ", serviceProbeURL)
	}
}
