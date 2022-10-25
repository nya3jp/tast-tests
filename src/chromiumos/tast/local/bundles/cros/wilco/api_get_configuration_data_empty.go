// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetConfigurationDataEmpty,
		Desc: "Test sending GetConfigurationData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon when not enrolled",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetConfigurationDataEmpty(ctx context.Context, s *testing.State) {
	request := dtcpb.GetConfigurationDataRequest{}
	response := dtcpb.GetConfigurationDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetConfigurationData", &request, &response); err != nil {
		s.Fatal("Unable to get configuration data: ", err)
	}

	// Error conditions defined by the proto definition.
	if response.JsonConfigurationData != "" {
		s.Fatalf("Unexpected GetConfigurationDataResponse; got %s, want an empty response", response.String())
	}
}
