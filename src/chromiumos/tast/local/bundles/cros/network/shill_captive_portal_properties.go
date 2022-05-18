// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type captivePortalProperties struct {
	serviceTechnology string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalProperties,
		Desc:     "Verifies that properties related to Captive portal are the expected values",
		Contacts: []string{"michaelrygiel@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "ethernet_online_with_no_captive_portal",
			Val: &captivePortalProperties{
				serviceTechnology: shillconst.TypeEthernet,
			},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func ShillCaptivePortalProperties(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)

	// Get Service Properties.
	params := s.Param().(*captivePortalProperties)
	props := map[string]interface{}{
		shillconst.ServicePropertyType: params.serviceTechnology,
	}
	service, err := manager.FindMatchingService(ctx, props)
	if err != nil {
		s.Fatal("Failed to find Ethernet Service: ", err)
	}
	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Service properties: ", err)
	}

	if err := verifyStringProperty(serviceProps, shillconst.ServicePropertyState, shillconst.ServiceStateOnline); err != nil {
		s.Fatal("Failed to verify string property: ", err)
	}
}

func verifyStringProperty(props *dbusutil.Properties, property, expectedValue string) error {
	value, err := props.GetString(property)
	if err != nil {
		return errors.Wrapf(err, "failed to get property %q", property)
	}
	if value != expectedValue {
		return errors.Errorf("unexpected %q property value: got %q, want: %q", property, value, expectedValue)
	}
	return nil
}
