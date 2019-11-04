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
		Func: ECVM,
		Desc: "Tests that the Wilco DTC VM can receive EC events",
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

func ECVM(ctx context.Context, s *testing.State) {
	// Create a channel to signal when the DPSL testing utility has received the
	// EC message inside of the VM.
	ch := make(chan struct{}, 1)
	msg := dtcpb.HandleEcNotificationRequest{}

	go func() {
		if err := wilco.DPSLReceiveMessage(ctx, ch, &msg); err != nil {
			s.Fatal("Unable to receive EC response: ", err)
		}
		close(ch)
	}()

	// Wait until the DPSLReceiveMessage function is listening to events inside
	// of the VM.
	select {
	case <-ch:
		s.Log("DPSLReceiveMessage ready")
	case <-ctx.Done():
		s.Fatal("Timout waiting for DPSLReceiveMessage to be Ready: ", ctx.Err())
	}

	if err := wilco.TriggerECEvent(); err != nil {
		s.Fatal("Unable to trigger EC event: ", err)
	}

	// Wait until the message has been received and the channel is closed.
	select {
	case <-ch:
		s.Logf("Message Received: %s", msg.String())
	case <-ctx.Done():
		s.Fatal("Timout waiting for EC event message: ", ctx.Err())
	}
}
