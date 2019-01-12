// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"context"

	"chromiumos/tast/local/bundles/cros/usbprinter/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddBasic,
		Desc:         "Verifies setup of a basic USB printer",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups", "usbip", "virtual-usb-printer"},
	})
}

const (
	addBasicAction          = "add"
	addBasicVid             = "04a9"
	addBasicPid             = "27e8"
	addBasicDescriptorsPath = "/var/lib/misc/usb_printer.json"
)

func AddBasic(ctx context.Context, s *testing.State) {
	conf := setup.NewConfig(addBasicAction, addBasicVid, addBasicPid,
		addBasicDescriptorsPath)

	printer, err := setup.Printer(ctx, conf)
	defer printer.Wait()
	defer printer.Kill()

	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
}
