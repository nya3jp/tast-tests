// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetOsVersion,
		Desc: "Test sending GetOsVersion gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetOsVersion(ctx context.Context, s *testing.State) {
	s.Log("Connecting to the D-Bus")
	_, obj, err := dbusutil.Connect(ctx, "org.bluez", "/")
	if err != nil {
		s.Fatal("Failed to connect to D-Bus: ", err)
	}
	value, err := dbusutil.GetManagedObjects(ctx, obj)
	if err != nil {
		s.Fatal("Failed get managed D-Bus objects: ", err)
	}
	s.Log("Managed D-Bus objects: ", value)
	s.Logf("Managed D-Bus objects: %T", value)

	m, ok := value.(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	if !ok {
		s.Fatal("Cannot convert to map")
	}

	for objPath, v := range m {
		for iface, vv := range v {
			s.Log(objPath, " ", iface, " ", vv)
		}
	}

	request := dtcpb.GetOsVersionRequest{}
	response := dtcpb.GetOsVersionResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetOsVersion", &request, &response); err != nil {
		s.Fatal("Unable to get OS version: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(response.Version) == 0 {
		s.Fatal(errors.Errorf("OS Version is blank: %s", response.String()))
	}
	if response.Milestone == 0 {
		s.Fatal(errors.Errorf("OS Milestone is 0: %s", response.String()))
	}
}
