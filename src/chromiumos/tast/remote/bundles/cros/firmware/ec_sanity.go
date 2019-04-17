// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/ec"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ECSanity,
		Desc:     "Check Google EC support",
		Contacts: []string{"ningappa.tirakannavar@intel.com",
                                   	"kasaiah.bogineni@intel.com",
                                  },
		Attr:     []string{"informational"},
	})
}

func ECSanity(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Failed to obtain servo: ", err)
	}
	chromeEC := ec.New(svo)
	var regexps = []string{`Chip:\s+([^\r\n]*)\r\n`, `RO:\s+([^\r\n]*)\r\n`, `RW_?[AB]?:\s+([^\r\n]*)\r\n`, `Build:\s+([^\r\n]*)\r\n`}
	val, err := chromeEC.Output(ctx, "version", regexps)
	if err != nil {
		s.Fatal("Failed to get version : ", err)
	}
	s.Logf("EC version is %s", val)
	if val == "None" {
		s.Error("EC console not enabled")
	}
	if _, err := d.Run(ctx, "ectool version"); err != nil {
		s.Error("No support for Google EC")
	}
}
