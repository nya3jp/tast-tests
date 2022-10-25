// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIHandleECNotification,
		Desc: "Tests that the Wilco DTC VM receives EC events using the DPSL",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIHandleECNotification(ctx context.Context, s *testing.State) {
	const (
		// Message type defined at http://issuetracker.google.com/139017129.
		expectedRequestType = 19
	)

	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		s.Fatal("Unable to create DPSL Message Receiver: ", err)
	}
	defer rec.Stop(ctx)

	// Give Stop time to clean up.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := wilco.TriggerECEvent(); err != nil {
		s.Fatal("Unable to trigger EC event: ", err)
	}

	for {
		s.Log("Waiting for EC Notification")
		msg := dtcpb.HandleEcNotificationRequest{}
		if err := rec.WaitForMessage(ctx, &msg); err != nil {
			s.Fatal("Unable to receive EC response: ", err)
		}

		if msg.Type == expectedRequestType {
			break
		}
		s.Logf("Received unexpected EC Notification: got %v; want %v. Continuing", msg.Type, expectedRequestType)
	}
}
