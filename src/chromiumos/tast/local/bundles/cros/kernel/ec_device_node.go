// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECDeviceNode,
		Desc: "Checks that device node for the primary embedded controller exists",
		Contacts: []string{
			"chromeos-chatty-kernel@google.com",
			"chromeos-kernel-test@google.com",
			"chromeos-kernel-team@google.com",
		},
		// TODO(kmshelton): Don't assume that /dev/cros_ec should exist on all devices.  A small part of the support
		// matrix does not have a CrOS EC, so this test won't be ready for mainline until the long tail of the support
		// matrix is accounted for (may need boxster EC data in-place to do this, see b/173741162).
		Attr: []string{"group:mainline", "informational"},
	})
}

// ECDeviceNode confirms that /dev/cros_ec (the device node for the primary embedded controller) exists.  If this test fails
// in the CQ, it's likely to be related to a kernel change and not a change in the primary embedded controller's firmware, as
// EC firmware for the quota pool (which backs the CQ) is changed only after a candidate build goes through FAFT (which would
// mean the EC is likely to be behaving as a responder to I2C or SPI messages).  Note that this test is similar to
// firmware.ECVersion, but doesn't depend on ectool or the EC reporting it's version in any particular format, so a failure is
// likely to be associated with a kernel change.  See
// https://chromium.googlesource.com/chromiumos/third_party/kernel/+/bd0447f72de0963129612bf18202204d5b25e133/ for an example
// where a change to a SPI driver prevented /dev/cros_ec from being created (ultimately discovered when flashrom invocations
// involving the EC failed).
func ECDeviceNode(ctx context.Context, s *testing.State) {
	// It is unknown whether the expectation of the creation for /dev/cros_ec is a longstanding convention, or if there is
	// any formal requirement that the kernel name the device node for the primary EC in this fashion.
	// TODO(kmshelton): Investigate whether Zephyr based EC's will use the "cros_ec" nomenclature.
	const ECDeviceNode = "/dev/cros_ec"
	// Checking the existence of the device node is selected over inspection of the kernel log for something like "Chrome EC device
	// registered," as it is anticipated that changes to the device node nomenclature are rarer than logging tweaks.
	_, err := os.Stat(ECDeviceNode)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.Fatal("The primary embedded controller's device node does not exist: ", err)
		}
		s.Fatal("Failed to stat the primary embedded controller's device node: ", err)
	}
}
