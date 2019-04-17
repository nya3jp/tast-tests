// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckEC,
		Desc:     "Check Google EC support",
		Contacts: []string{"ningappa.tirakannavar@intel.com"},
		Attr:     []string{"disabled", "informational"},
	})
}

//CheckEC : Method to Check the EC console enabled or not and also check google EC support.
func CheckEC(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}
	var regexps = []string{"Chip:\\s+([^\\r\\n]*)\\r\\n", "RO:\\s+([^\\r\\n]*)\\r\\n", "RW_?[AB]?:\\s+([^\\r\\n]*)\\r\\n", "Build:\\s+([^\\r\\n]*)\\r\\n"}
	val, err := svo.SendEcCommandAndGetOutput(ctx, "version", regexps)
	if err != nil {
		s.Fatalf("Failed in SendEcCommandAndGetOutput, command is %s. Error is %v", "version", err)
	}
	if val == "None" {
		s.Error("EC console not enabled")
	}
	s.Logf("Ec version is %s", val)
	_, err = d.Run(ctx, "ectool version")
	if err != nil {
		s.Error("No support for Google EC")
	}
}
