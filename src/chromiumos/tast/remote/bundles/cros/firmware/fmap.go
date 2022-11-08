// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fmap,
		Desc:         "Verify FMAP displays expected information",
		Contacts:     []string{"tij@google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "ec",
				Val:     pb.Programmer_ECProgrammer,
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "bios",
				Val:     pb.Programmer_BIOSProgrammer,
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "ec_dev",
				Val:     pb.Programmer_ECProgrammer,
				Fixture: fixture.DevModeGBB,
			},
			{
				Name:    "bios_dev",
				Val:     pb.Programmer_BIOSProgrammer,
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

func Fmap(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	programmer := s.Param().(pb.Programmer)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to require configs: ", err)
	}
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	/*
		Expected ec Structure:
			WP_RO
				|--> EC_RO
						|--> FMAP
						|--> RO_FRID
			EC_RW
				|--> RW_FWID
	*/
	ecExpectedStructure := map[pb.ImageSection][]pb.ImageSection{
		pb.ImageSection_APWPROImageSection: []pb.ImageSection{pb.ImageSection_ECROImageSection},
		pb.ImageSection_ECRWImageSection:   []pb.ImageSection{pb.ImageSection_RWFWIDImageSection},
		pb.ImageSection_RWFWIDImageSection: []pb.ImageSection{},
		pb.ImageSection_ECROImageSection:   []pb.ImageSection{pb.ImageSection_FMAPImageSection, pb.ImageSection_ROFRIDImageSection},
		pb.ImageSection_FMAPImageSection:   []pb.ImageSection{},
		pb.ImageSection_ROFRIDImageSection: []pb.ImageSection{},
	}

	/*
		Expected host Structure:
			RW_VPD
			WP_RO
				|--> RO_SECTION
				|--> RO_VPD
						|--> FMAP
						|--> RO_FRID
						|--> GBB
			RW_SECTION_A
				|--> VBLOCK_A
				|--> FW_MAIN_A
				|--> RW_FWID_A
				|--> ME_RW_A    (Only if DUT uses INTEL CSE LITE FMAP Scheme)
			RW_SECTION_B
				|--> VBLOCK_B
				|--> FW_MAIN_B
				|--> RW_FWID_B
				|--> ME_RW_B    (Only if DUT uses INTEL CSE LITE FMAP Scheme)
	*/
	hostExpectedStructure := map[pb.ImageSection][]pb.ImageSection{
		pb.ImageSection_RWVPDImageSection:  []pb.ImageSection{},
		pb.ImageSection_APWPROImageSection: []pb.ImageSection{pb.ImageSection_APROImageSection, pb.ImageSection_ROVPDImageSection},
		pb.ImageSection_ROVPDImageSection:  []pb.ImageSection{},
		pb.ImageSection_APROImageSection:   []pb.ImageSection{pb.ImageSection_FMAPImageSection, pb.ImageSection_ROFRIDImageSection, pb.ImageSection_GBBImageSection},
		pb.ImageSection_FMAPImageSection:   []pb.ImageSection{},
		pb.ImageSection_ROFRIDImageSection: []pb.ImageSection{},
		pb.ImageSection_GBBImageSection:    []pb.ImageSection{},
		pb.ImageSection_APRWAImageSection: []pb.ImageSection{
			pb.ImageSection_FWSignAImageSection,
			pb.ImageSection_FWBodyAImageSection,
			pb.ImageSection_RWFWIDAImageSection,
		},
		pb.ImageSection_APRWBImageSection: []pb.ImageSection{
			pb.ImageSection_FWSignBImageSection,
			pb.ImageSection_FWBodyBImageSection,
			pb.ImageSection_RWFWIDBImageSection,
		},
		pb.ImageSection_FWSignAImageSection: []pb.ImageSection{},
		pb.ImageSection_FWBodyAImageSection: []pb.ImageSection{},
		pb.ImageSection_RWFWIDAImageSection: []pb.ImageSection{},
		pb.ImageSection_FWSignBImageSection: []pb.ImageSection{},
		pb.ImageSection_FWBodyBImageSection: []pb.ImageSection{},
		pb.ImageSection_RWFWIDBImageSection: []pb.ImageSection{},
	}

	parsedFmap, err := h.BiosServiceClient.ParseFMAP(ctx, &pb.FMAP{Programmer: programmer})
	if err != nil {
		s.Fatal("Failed to save current fw image: ", err)
	}
	fmap := make(map[pb.ImageSection]*pb.Range, len(parsedFmap.Fmap))
	s.Log("Parsed FMAP Ranges:")
	for _, v := range parsedFmap.Fmap {
		fmap[v.Section] = v.Range
		s.Logf("\t%v: {Start: %v, Len: %v}", v.Section, fmap[v.Section].Start, fmap[v.Section].Length)
	}

	switch programmer {
	case pb.Programmer_BIOSProgrammer:
		for region := range hostExpectedStructure {
			if _, ok := fmap[region]; !ok {
				s.Fatalf("Expected region %v to be in host fmap but was not present", region)
			}
			if fmap[region].Length == 0 {
				s.Fatalf("Expected section %v to have non 0 size", region)
			}
		}

		if fmap[pb.ImageSection_APRWAImageSection].Length != fmap[pb.ImageSection_APRWBImageSection].Length {
			s.Fatal("Expected RW_SECTION_A and RW_SECTION_B to be the same size")
		}

		if fmap[pb.ImageSection_RWLEGACYImageSection].Length < 1024*1024 {
			s.Fatal("Expected RW_LEGACY to have size >= 1 MB")
		}

		if h.Config.SMMStore {
			out, err := h.DUT.Conn().CommandContext(ctx, "uname", "-m").Output(ssh.DumpLogOnError)
			if err != nil {
				s.Fatal("Failed to get uname from dut")
			}
			if strings.Contains(string(out), "x86") {
				if _, ok := fmap[pb.ImageSection_SMMSTOREImageSection]; !ok {
					s.Fatal("Expected region SMMSTORE to be in host fmap but was not present")
				} else if fmap[pb.ImageSection_SMMSTOREImageSection].Length < 256*1024 {
					s.Fatal("Expected SMMTORE to have size >= 256 KB")
				}
			}
		}

		// If actual fmap has ME_RW_A (implies also has ME_RW_B) add those to expected structure
		if _, ok := fmap[pb.ImageSection_IntelCSERWAImageSection]; ok && programmer == pb.Programmer_ECProgrammer {
			alst := hostExpectedStructure[pb.ImageSection_APRWAImageSection]
			alst = append(alst, pb.ImageSection_IntelCSERWAImageSection)
			hostExpectedStructure[pb.ImageSection_APRWAImageSection] = alst
			hostExpectedStructure[pb.ImageSection_IntelCSERWAImageSection] = []pb.ImageSection{}

			blst := hostExpectedStructure[pb.ImageSection_APRWBImageSection]
			blst = append(blst, pb.ImageSection_IntelCSERWBImageSection)
			hostExpectedStructure[pb.ImageSection_APRWBImageSection] = blst
			hostExpectedStructure[pb.ImageSection_IntelCSERWBImageSection] = []pb.ImageSection{}
		}

		if err := checkStructure(ctx, fmap, hostExpectedStructure); err != nil {
			s.Fatal("Output of host FMAP didn't match expected structure: ", err)
		}
	case pb.Programmer_ECProgrammer:
		for region := range ecExpectedStructure {
			if _, ok := fmap[region]; !ok {
				s.Fatalf("Expected region %v to be in ec fmap but was not present", region)
			}
			if fmap[region].Length == 0 {
				s.Fatalf("Expected section %v to have non 0 size", region)
			}
		}

		if err := checkStructure(ctx, fmap, ecExpectedStructure); err != nil {
			s.Fatal("Output of host FMAP didn't match expected structure: ", err)
		}
	}
}

func checkStructure(ctx context.Context, fmap map[pb.ImageSection]*pb.Range, target map[pb.ImageSection][]pb.ImageSection) error {
	testInsideBound := func(parentRegion, childRegion *pb.Range) bool {
		return childRegion.Start >= parentRegion.Start &&
			childRegion.Start+childRegion.Length <= parentRegion.Start+parentRegion.Length
	}

	for parent, children := range target {
		parentRange := fmap[parent]
		childRanges := make([]*pb.Range, len(children))
		testing.ContextLogf(ctx, "Checking structure for section: %v with range: %v, with children %v", parent, parentRange, children)
		for i, child := range children {
			// Verify each child is within parent's bounds
			if !testInsideBound(parentRange, fmap[child]) {
				return errors.Errorf("expected section %v to be contained within section %v, but was not", child, parent)
			}
			childRanges[i] = fmap[child]
		}

		// Sort children ranges by start so not every pair of children have to be .
		sort.Slice(childRanges, func(i, j int) bool {
			return childRanges[i].Start < childRanges[j].Start
		})

		// Check adjacent ranges for overlap.
		for i := 1; i < len(childRanges); i++ {
			prev := childRanges[i-1]
			if prev.Start+prev.Length > childRanges[i].Start {
				return errors.Errorf("One of the children ranges of section %v overlap unexpectedly", parent)
			}
		}
	}
	return nil
}
