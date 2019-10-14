// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"time"

	wvm "chromiumos/tast/local/bundles/cros/wilco/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECEvent,
		Desc: "Tests that userspace can receive events from the EC on Wilco devices",
		Contacts: []string{
			"ncrews@chromium.org",       // Test author and EC kernel driver author.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel-test@google.com",
		},
		SoftwareDeps: []string{"wilco"},
		Timeout:      10 * time.Second,
		Attr:         []string{"group:mainline", "informational"},
	})
}

// ECEvent tests the Wilco EC is able to generate EC events and that the kernel
// properly passes them along to userspace to read. Normally, the EC triggers
// events on hardware changes, such as battery errors. For testing, we can
// send a special command to the EC, and the EC will generate a hardcoded dummy
// event. This EC feature is only present on EC FW > 01.00.03, so skip the
// test on devices with old EC FW.
//
// See http://chromium.googlesource.com/chromiumos/third_party/kernel/+/283563c976eefc1ab2e83049665d42b23bda95b5/drivers/platform/chrome/wilco_ec/debugfs.c#221
// for the kernel driver that sends the test event command, and see
// http://chromium.googlesource.com/chromiumos/third_party/kernel/+/283563c976eefc1ab2e83049665d42b23bda95b5/drivers/platform/chrome/wilco_ec/event.c
// for the kernel driver that reads events from the EC.
func ECEvent(ctx context.Context, s *testing.State) {
	const (
		eventTriggerPath = "/sys/kernel/debug/wilco_ec/test_efsdfvent"
		eventReadPath    = "/dev/wilco_event0"
		testEventSize    = 16
	)
	// The format of this dummy event chosen at
	// http://issuetracker.google.com/139017129.
	var expectedECEvent = [...]byte{
		0x07, 0x00, 0x13, 0x00, 0x00, 0x00, 0x01, 0x00, 0x02, 0x00,
		0x03, 0x00, 0x04, 0x00, 0x05, 0x00}

	if _, err := os.Stat(eventTriggerPath); os.IsNotExist(err) {
		s.Logf("%v does not exist. The EC FW does not support test events. Skipping the test", eventTriggerPath)
		return
	}

	// We need exclusive access to |eventReadPath|, so ensure that
	// wilco_dtc_supportd that controls it is shut down. No need to restart
	// it when we're done.
	if err := wvm.StopWilcoSupportDaemon(ctx); err != nil {
		s.Fatal("Unable to stop wilco_dtc_supportd: ", err)
	}

	// Writing anything to |eventTriggerPath| should cause the EC to
	// generate the dummy event.
	if err := ioutil.WriteFile(eventTriggerPath, []byte{0}, 0644); err != nil {
		s.Fatalf("Failed to write to %v: %v", eventTriggerPath, err)
	}

	f, err := os.OpenFile(eventReadPath, os.O_RDONLY, 0644)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", eventReadPath, err)
	}
	defer f.Close()

	// There could already be other EC events enqueued in the kernel
	// waiting to be read, so read more than maybe necessary to ensure
	// that what we read includes the dummy event.
	var readEvents [4 * testEventSize]byte
	s.Log("Performing a blocking read of", eventReadPath)
	_, err = f.Read(readEvents[:])
	if err != nil {
		s.Fatalf("Failed to read from %v: %v", eventReadPath, err)
	}

	if !bytes.Contains(readEvents[:], expectedECEvent[:]) {
		s.Fatalf("The bytes read [% #x] do not contain the expected EC Event [% #x]", readEvents, expectedECEvent)
	}
}
