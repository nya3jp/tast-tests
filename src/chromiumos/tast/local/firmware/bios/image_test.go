// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bios

import (
	"testing"

	pb "chromiumos/tast/services/cros/firmware"
)

func TestCalcGBB(t *testing.T) {
	// 1 bit
	m := calcGBBMask([]pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY})
	if m != 0x0001<<pb.GBBFlag_DEV_SCREEN_SHORT_DELAY {
		t.Fatalf("unexpected mask for 1 bit: %v", m)
	}

	f := calcGBBFlags(m)
	if len(f) != 1 || f[0] != pb.GBBFlag_DEV_SCREEN_SHORT_DELAY {
		t.Fatalf("unexpected flagfor 1 bit: %v", f)
	}

	// 2 bits
	m = calcGBBMask([]pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY, pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP})

	if m != (0x0001<<pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)|(0x0001<<pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP) {
		t.Fatalf("unexpected mask for 2 bits: %v", m)
	}

	f = calcGBBFlags(m)
	if len(f) != 2 || f[0] != pb.GBBFlag_DEV_SCREEN_SHORT_DELAY || f[1] != pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP {
		t.Fatalf("unexpected flags for 2 bits: %v", f)
	}
}

func TestSectionData(t *testing.T) {
	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{1, 16}}
	i := Image{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, s}
	var flag uint32
	err := i.readSectionData(GBBImageSection, 12, 4, &flag)
	if err != nil {
		t.Fatal(err)
	}
	if flag != 0x04030201 {
		t.Fatalf("unexpected flags read %x from image %v", flag, i)
	}
}

func TestShortGBBSection(t *testing.T) {
	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{0, 15}}
	i := Image{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, s}
	var flag uint32
	err := i.readSectionData(GBBImageSection, 12, 4, &flag)
	if err == nil {
		t.Fatal("Short section not detected: ", err)
	}
}

func TestGetGBBFlags(t *testing.T) {
	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{1, 16}}
	i := Image{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01, 0x01, 0, 0, 0xff}, s}
	cf, sf, err := i.GetGBBFlags()
	if err != nil {
		t.Fatalf("failed to perform GetGBBFlags: %v", err)
	}
	if len(cf) != len(pb.GBBFlag_name)-2 {
		t.Errorf("cleared flags count incorrect, wanted %v, got %v: %v", len(pb.GBBFlag_name)-2, len(cf), cf)
	}
	if len(sf) != 2 {
		t.Fatalf("set flags count incorrect, wanted 2, got %v: %v", len(sf), sf)
	}
	if int(sf[0]) != 0 {
		t.Fatalf("1st set flag incorrect: %v", sf)
	}
	if int(sf[1]) != 8 {
		t.Fatalf("2nd set flag incorrect: %v", sf)
	}
}
