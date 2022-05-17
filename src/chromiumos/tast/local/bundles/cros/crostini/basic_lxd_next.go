// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicLxdNext,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests Crostini starts up with LXD 4.0",
		Contacts:     []string{"sidereal@google.com", "clumptini@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseyeWithLxdNext",
				Timeout:           7 * time.Minute,
			}, {
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Fixture:           "crostiniBullseyeWithLxdNext",
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func BasicLxdNext(ctx context.Context, s *testing.State) {
	cont := s.FixtValue().(crostini.FixtureData).Cont

	r := regexp.MustCompile("^Client version: 4.0.[0-9]+\nServer version: 4.0.[0-9]+\n$")

	cmd := cont.VM.Command(ctx,
		// These variables get set *outside* the VM by crostini_client, so we have to add them manually here.
		"LXD_DIR=/mnt/stateful/lxd", "LXD_CONF=/mnt/stateful/lxd_conf",
		// Setting -i tricks the bashrc file into thinking we're an interactive session.
		"bash", "-i", "-c", "lxc version")
	stdout, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run command: ", err)
	} else if !r.Match(stdout) {
		s.Fatal("Unexpected output: ", string(stdout))
	}
}
