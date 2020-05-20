// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbc

import (
	"context"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/remote/bundles/mtbf/usbc/common"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     USBControl,
		Desc:     "Controll USB by Allion",
		Contacts: []string{"xliu@cienet.com"},
		Params: []testing.Param{{
			Name: "on",
			Val:  "on",
		}, {
			Name: "off",
			Val:  "off",
		}},
		Vars: []string{"allion.api.server", "allion.usb.deviceId"},
		Attr: []string{"group:mainline", "informational"},
	})
}

// USBControl control USB by Allion
func USBControl(ctx context.Context, s *testing.State) {
	usbSwitch := s.Param().(string)
	allionServerURL := common.GetVar(ctx, s, "allion.api.server")
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	deviceID := common.GetVar(ctx, s, "allion.usb.deviceId")

	if usbSwitch == "on" {
		s.Log("Start to enable USB")
		if mtbferr := allionAPI.EnableUsb(deviceID); mtbferr != nil {
			s.Fatal(mtbferr)
		}
	} else {
		s.Log("Start to disable USB")
		if mtbferr := allionAPI.DisableUsb(deviceID); mtbferr != nil {
			s.Fatal(mtbferr)
		}
	}
}
