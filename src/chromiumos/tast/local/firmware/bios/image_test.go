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

func TestReadSectionData(t *testing.T) {
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

func TestWriteSectionData(t *testing.T) {
	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{1, 16}}
	i := Image{[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}, s}

	var flag uint32
	flag = 0x04030201
	if err := i.writeSectionData(GBBImageSection, 12, flag); err != nil {
		t.Fatal(err)
	}

	var got [18]byte
	copy(got[:], i.data[:])

	want := [18]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 1, 2, 3, 4, 18}

	if got != want {
		t.Fatalf("image data incorrect, got: %v, want: %v", got, want)
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

func TestClearSetGBBFlags(t *testing.T) {
	beforeBytes := [13]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	afterBytes := [1]byte{14}
	dataSlice := make([]byte, 18)
	copy(dataSlice[0:13], beforeBytes[:])
	copy(dataSlice[17:18], afterBytes[:])

	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{1, 16}}
	i := Image{dataSlice, s}

	if err := i.ClearSetGBBFlags([]pb.GBBFlag{}, []pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY, pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP}); err != nil {
		t.Fatal("failed to initially ClearSetGBBFlags: ", err)
	}
	cf, sf, err := i.GetGBBFlags()
	if err != nil {
		t.Fatalf("failed to initially perform GetGBBFlags: %v", err)
	}
	if len(cf) != len(pb.GBBFlag_name)-2 {
		t.Errorf("cleared initial flags count incorrect, wanted %v, got %v: %v", len(pb.GBBFlag_name)-2, len(cf), cf)
	}
	if len(sf) != 2 {
		t.Fatalf("set initial flags count incorrect, wanted 2, got %v: %v", len(sf), sf)
	}
	if sf[0] != pb.GBBFlag_DEV_SCREEN_SHORT_DELAY {
		t.Fatalf("1st set initial flag incorrect: %v", sf)
	}
	if sf[1] != pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP {
		t.Fatalf("2nd set initial flag incorrect: %v", sf)
	}

	if err := i.ClearSetGBBFlags([]pb.GBBFlag{pb.GBBFlag_DEV_SCREEN_SHORT_DELAY}, []pb.GBBFlag{pb.GBBFlag_DISABLE_LID_SHUTDOWN}); err != nil {
		t.Fatal("failed to ClearSetGBBFlags: ", err)
	}
	cf, sf, err = i.GetGBBFlags()
	if err != nil {
		t.Fatalf("failed to perform GetGBBFlags: %v", err)
	}
	if len(cf) != len(pb.GBBFlag_name)-2 {
		t.Errorf("cleared flags count incorrect, wanted %d, got %d: %v", len(pb.GBBFlag_name)-2, len(cf), cf)
	}
	if len(sf) != 2 {
		t.Fatalf("set flags count incorrect, wanted 2, got %d: %v", len(sf), sf)
	}
	if sf[0] != pb.GBBFlag_DISABLE_LID_SHUTDOWN {
		t.Fatalf("1st set flag incorrect: %v", sf)
	}
	if sf[1] != pb.GBBFlag_FORCE_DEV_BOOT_FASTBOOT_FULL_CAP {
		t.Fatalf("2nd set flag incorrect: %v", sf)
	}

	var resBeforeBytes [13]byte
	var resAfterBytes [1]byte

	copy(resBeforeBytes[:], i.data[:13])
	copy(resAfterBytes[:], i.data[17:])

	if resBeforeBytes != beforeBytes {
		t.Fatalf("bytes before GBB header changed, got %v, want %v", resBeforeBytes, beforeBytes)
	}
	if resAfterBytes != afterBytes {
		t.Fatalf("bytes after GBB header changed, got %v, want %v", resAfterBytes, afterBytes)
	}
}

func TestGetLayout(t *testing.T) {
	s := map[ImageSection]SectionInfo{GBBImageSection: SectionInfo{1, 16}}
	i := Image{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, s}
	expectedLayout := "0x00000001:0x00000010 FV_GBB\n"
	layout := string(i.getLayout())
	if layout != expectedLayout {
		t.Fatalf("unexpected layout, want %s, got %s", expectedLayout, layout)
	}
}
