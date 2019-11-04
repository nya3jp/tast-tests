// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: APIHandleECNotification,
		Desc: "Tests that the Wilco DTC VM receives EC events using the DPSL",
		Contacts: []string{
			"tbegin@chromium.org",       // Test author and Wilco DTC VM author.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
		},
		SoftwareDeps: []string{"wilco"},
		Timeout:      10 * time.Second,
		Attr:         []string{"group:mainline", "informational"},
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
		s.Fatal("Unable to create DPSL Message Receiver")
	}
	defer rec.Stop()

	if err := wilco.TriggerECEvent(); err != nil {
		s.Fatal("Unable to trigger EC event: ", err)
	}

	s.Log("Waiting for EC Notification")
	msg := dtcpb.HandleEcNotificationRequest{}
	if err := rec.WaitForMessage(ctx, &msg); err != nil {
		s.Fatal("Unable to receive EC response: ", err)
	}
	s.Log("Received EC Notification")

	if msg.Type != expectedRequestType {
		s.Fatalf("EC Notification Request is the wrong type. Got %v, expected %v",
			msg.Type, expectedRequestType)
	}
}
