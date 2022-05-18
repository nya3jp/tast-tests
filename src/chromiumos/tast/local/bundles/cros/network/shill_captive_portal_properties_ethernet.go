// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalPropertiesEthernet,
		Desc:     "Verifies that properties related to Captive portal are the expected values on an online ethernet connection",
		Contacts: []string{"michaelrygiel@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillCaptivePortalPropertiesEthernet(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)

	// Get Manager Properties
	managerProps, err := manager.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get the shill manager properties: ", err)
	}
	verifyStringProperty(managerProps, shillconst.ManagerPropertyCheckPortalList, shillconst.ManagerCheckPortalListNone, s)

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

	verifyStringProperty(serviceProps, shillconst.ServicePropertyCheckPortal, shillconst.ServiceCheckPortalAuto, s)
	verifyStringProperty(serviceProps, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, s)
	verifyStringProperty(serviceProps, shillconst.ServicePropertyPortalDetectionFailedPhase, shillconst.ServicePortalDetectionFailedPhaseNone, s)
	verifyStringProperty(serviceProps, shillconst.ServicePropertyPortalDetectionFailedStatus, shillconst.ServicePortalDetectionFailedStatusNone, s)
	verifyInt32Property(serviceProps, shillconst.ServicePropertyPortalDetectionFailedStatusCode, shillconst.ServicePortalDetectionFailedStatusCodeZero, s)
	verifyStringProperty(serviceProps, shillconst.ServicePropertyProbeURL, shillconst.ServiceProbeURLNone, s)
}

func verifyStringProperty(props *dbusutil.Properties, property, expectedValue string, s *testing.State) {
	value, err := props.GetString(property)
	if err != nil {
		s.Fatalf("Failed to get property: %v, err: %v", property, err)
	}
	if value != expectedValue {
		s.Fatalf("Property (%v) value is different from expected. GOT: %v, EXPECTED: %v", property, value, expectedValue)
	}
}

func verifyInt32Property(props *dbusutil.Properties, property string, expectedValue int32, s *testing.State) {
	value, err := props.GetInt32(property)
	if err != nil {
		s.Fatalf("Failed to get property: %v, err: %v", property, err)
	}
	if value != expectedValue {
		s.Fatalf("Property (%v) value is different from expected. GOT: %v, EXPECTED: %v", property, value, expectedValue)
	}
}
