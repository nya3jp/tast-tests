// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"reflect"
	"testing"
)

func TestParseGetStorageInfoOutputSimpleHealthyEMMC(t *testing.T) {
	const out = `
  Extended CSD rev 1.8 (MMC 5.1)
Pre EOL information [PRE_EOL_INFO: 0x01]
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "EMMC",
		Device: EMMC,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleFailingEMMC(t *testing.T) {
	const out = `
  Extended CSD rev 1.8 (MMC 5.1)
Pre EOL information [PRE_EOL_INFO: 0x03]
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "EMMC",
		Device: EMMC,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleOlderEMMC(t *testing.T) {
	const out = `
  Extended CSD rev 0.0 (MMC 4.5)
Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x00]
Pre EOL information [PRE_EOL_INFO: 0x01]
`
	info, err := parseGetStorageInfoOutput([]byte(out))
	if err == nil {
		t.Fatal("parseGetStorageInfoOutput() passed, but should have failed: ", info)
	}
}

func TestParseGetStorageInfoOutputSimpleHealthyNVMe(t *testing.T) {
	const out = `
	SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
	Available Spare:			100%
	Available Spare Threshold:		10%
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "NVME",
		Device: NVMe,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleFailingNVMe(t *testing.T) {
	const out = `
	SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
	Available Spare:			9%
	Available Spare Threshold:		10%
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "NVME",
		Device: NVMe,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleHealthySSD(t *testing.T) {
	const out = `
	ATA Version is:   7
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   -O----   100   100   000    -    0
  9 Power_On_Hours          -O----   100   100   000    -    333
 12 Power_Cycle_Count       -O----   100   100   000    -    1758
165 Total_Write/Erase_Count -O----   100   100   000    -    174113
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "SATA",
		Device: SSD,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleFailingSSD(t *testing.T) {
	const out = `
	ATA Version is:   7
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   -O----   100   100   000    -    0
  9 Power_On_Hours          -O----   100   100   000    -    333
 12 Power_Cycle_Count       -O----   100   100   000    -    1758
165 Total_Write/Erase_Count -O----   100   100   000    NOW  174113
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "SATA",
		Device: SSD,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleHealthySSDUncorrect(t *testing.T) {
	const out = `
	ATA Version is:   7
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   -O----   100   100   000    -    0
  9 Power_On_Hours          -O----   100   100   000    -    333
 12 Power_Cycle_Count       -O----   100   100   000    -    1758
187 Reported_Uncorrect      -O----   100   100   000    -    0
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "SATA",
		Device: SSD,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleFailingSSDUncorrect(t *testing.T) {
	const out = `
	ATA Version is:   7
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
  5 Reallocated_Sector_Ct   -O----   100   100   000    -    0
  9 Power_On_Hours          -O----   100   100   000    -    333
 12 Power_Cycle_Count       -O----   100   100   000    -    1758
187 Reported_Uncorrect      -O----   100   100   000    -    1
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:   "SATA",
		Device: SSD,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}
