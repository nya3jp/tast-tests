// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECEvent,
		Desc: "Tests that userspace can receive events from the EC on Wilco devices",
		Contacts: []string{
			"campello@chromium.org",     // Test maintainer.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel-test@google.com",
		},
		SoftwareDeps: []string{"wilco"},
		Timeout:      10 * time.Second,
		Attr:         []string{"group:mainline"},
	})
}

// ECEvent tests the Wilco EC is able to generate EC events and that the kernel
// properly passes them along to userspace to read. Normally, the EC triggers
// events on hardware changes, such as battery errors. For testing, we can
// send a special command to the EC, and the EC will generate a hardcoded dummy
// event.
//
// See http://chromium.googlesource.com/chromiumos/third_party/kernel/+/283563c976eefc1ab2e83049665d42b23bda95b5/drivers/platform/chrome/wilco_ec/debugfs.c#221
// for the kernel driver that sends the test event command, and see
// http://chromium.googlesource.com/chromiumos/third_party/kernel/+/283563c976eefc1ab2e83049665d42b23bda95b5/drivers/platform/chrome/wilco_ec/event.c
// for the kernel driver that reads events from the EC.
func ECEvent(ctx context.Context, s *testing.State) {
	// The format of this dummy event chosen at
	// http://issuetracker.google.com/139017129.
	expectedECEvent := []byte{
		0x07, 0x00, 0x13, 0x00, 0x00, 0x00, 0x01, 0x00,
		0x02, 0x00, 0x03, 0x00, 0x04, 0x00, 0x05, 0x00}

	// We need exclusive access to |eventReadPath|, so ensure that
	// wilco_dtc_supportd that controls it is shut down. No need to restart
	// it when we're done.
	if err := wilco.StopSupportd(ctx); err != nil {
		s.Fatal("Unable to stop wilco_dtc_supportd: ", err)
	}

	// Before we start the test, clear the queue of events to ensure there are
	// no stale events.
	if err := wilco.ClearECEventQueue(ctx); err != nil {
		s.Fatal("Unable to clear wilco EC event queue: ", err)
	}

	if err := wilco.TriggerECEvent(); err != nil {
		s.Fatal("Unable to trigger EC event: ", err)
	}

	// Drain the queue of EC events and check each one for the expected payload.
	// Stop when the queue is empty, or we receive an error reading the EC
	// event. The maximum queue size is 16 so this is small enough to poll until
	// empty.
	for {
		readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second))
		defer cancel()

		s.Log("Performing blocking read of EC event file")
		event, err := wilco.ReadECEvent(readCtx)
		if err != nil {
			s.Fatal("Unable to read EC data: ", err)
		}
		if bytes.Contains(event, expectedECEvent) {
			s.Log("Received matching EC event")
			break
		}

		s.Logf("Received a different EC event. "+
			"The bytes read [% #x] do not contain the expected EC Event [% #x]. Continuing",
			event, expectedECEvent)

	}
}
