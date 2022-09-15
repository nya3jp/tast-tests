// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Login,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests Chrome Login with different VMs running",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "novm",
			Pre:  multivm.NoVMStarted(),
		}, {
			Name:              "arc_crostini",
			Pre:               multivm.ArcCrostiniStarted(),
			ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraSoftwareDeps: []string{"vm_host", "android_vm"},
		}, {
			Name:              "arc",
			Pre:               multivm.ArcStarted(),
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "arc_p",
			Pre:               multivm.ArcStarted(),
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "crostini",
			Pre:               multivm.CrostiniStarted(),
			ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraSoftwareDeps: []string{"vm_host"},
		}},
	})
}

const (
	postLoginCoolDownDuration = 10 * time.Second
	quietDuration             = 60 * time.Second
)

func Login(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)

	if err := pre.Chrome.Responded(ctx); err != nil {
		s.Fatal("Chrome did not respond: ", err)
	}
	arc := multivm.ARCFromPre(pre)
	basemem, err := metrics.NewBaseMemoryStats(ctx, arc)
	if err != nil {
		s.Fatal("Failed to retrieve base memory stats: ", err)
	}

	if arc != nil {
		// Ensures package manager service is running by checking the existence of the "android" package.
		pkgs, err := arc.InstalledPackages(ctx)
		if err != nil {
			s.Fatal("Getting installed packages failed: ", err)
		}

		if _, ok := pkgs["android"]; !ok {
			s.Fatal("Android package not found: ", pkgs)
		}
	}

	crostiniVM := multivm.CrostiniFromPre(pre)
	if crostiniVM != nil {
		if err := crostini.BasicCommandWorks(ctx, crostiniVM); err != nil {
			s.Fatal("Crostini basic commands don't work: ", err)
		}
	}

	p := perf.NewValues()
	if err := metrics.LogMemoryStats(ctx, basemem, arc, p, s.OutDir(), "_login"); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}

	// Cool down a little post login, before we start a quiet period.
	s.Log("No activity for a short while to cool down")
	testing.Sleep(ctx, postLoginCoolDownDuration)

	s.Log("Measuring system memory and pressure in idle state")
	basemem.Reset()
	// Let the system quiesce for a while and measure its memory consumption.
	testing.Sleep(ctx, quietDuration)
	s.Log("Will now collect idle perf values")
	if err := metrics.LogMemoryStats(ctx, basemem, arc, p, s.OutDir(), "_quiesce"); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf.Values: ", err)
	}
}
