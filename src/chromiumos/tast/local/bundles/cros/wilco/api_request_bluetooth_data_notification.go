// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wilco/bt"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIRequestBluetoothDataNotification,
		Desc: "Test sending RequestBluetoothDataNotification gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon and expect a response",
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

func APIRequestBluetoothDataNotification(ctx context.Context, s *testing.State) {
	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop(ctx)

	// Give Stop time to clean up.
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	request := dtcpb.RequestBluetoothDataNotificationRequest{}
	response := dtcpb.RequestBluetoothDataNotificationResponse{}

	// Repeat test to make sure it's not influenced by system events.
	for i := 0; i < 10; i++ {
		if err := wilco.DPSLSendMessage(ctx, "RequestBluetoothDataNotification", &request, &response); err != nil {
			s.Fatal("Unable to request notification: ", err)
		}

		for {
			s.Log("Waiting for Bluetooth event")
			msg := &dtcpb.HandleBluetoothDataChangedRequest{}
			if err := rec.WaitForMessage(ctx, msg); err != nil {
				s.Fatal("Unable to receive Bluetooth event: ", err)
			}

			if err := bt.ValidateBluetoothData(ctx, msg); err != nil {
				s.Logf("Unable to validate Bluetooth data %v: %v", msg, err)
			} else {
				break
			}
		}
	}
}
