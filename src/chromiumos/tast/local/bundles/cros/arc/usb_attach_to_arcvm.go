// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
  "context"

  "chromiumos/tast/common/testexec"
  "chromiumos/tast/errors"
  "chromiumos/tast/local/arc"
  "chromiumos/tast/testing"
)

func init() {
  testing.AddTest(&testing.Test{
    Func:         UsbAttachToArcvm,
    LacrosStatus: testing.LacrosVariantUnneeded,
    Desc:         "Validity attaching virtual usb device to arcvm",
    Contacts:     []string{"lgcheng@google.com", "arc-eng@google.com"},
    Attr:         []string{"group:mainline", "informational"},
    SoftwareDeps: []string{"chrome"},
    Params: []testing.Param{{
      Name:              "vm",
      ExtraSoftwareDeps: []string{"android_vm"},
    }},
  })
}

func UsbAttachToArcvm(ctx context.Context, s *testing.State) {

}

func AttachUsbDevice(ctx context.Context) {

}

func DetachUsbDevice(ctx context.Context) {

}
