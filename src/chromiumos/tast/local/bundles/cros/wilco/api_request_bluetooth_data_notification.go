// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIRequestBluetoothDataNotification,
		Desc: "Test sending RequestBluetoothDataNotification gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon and expect a response",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIRequestBluetoothDataNotification(ctx context.Context, s *testing.State) {
	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop()

	request := dtcpb.RequestBluetoothDataNotificationRequest{}
	response := dtcpb.RequestBluetoothDataNotificationResponse{}

	if err := wilco.DPSLSendMessage(ctx, "RequestBluetoothDataNotification", &request, &response); err != nil {
		s.Fatal("Unable to request notification: ", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	s.Log("Waiting for bluetooth event")
	msg := dtcpb.HandleBluetoothDataChangedRequest{}
	if err := rec.WaitForMessage(ctx, &msg); err != nil {
		s.Fatal("Unable to receive bluetooth event: ", err)
	}
}
