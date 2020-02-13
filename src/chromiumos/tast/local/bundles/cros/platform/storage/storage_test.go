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
Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x00]
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
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
Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x0A]
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
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
`
	info, err := parseGetStorageInfoOutput([]byte(out))
	if err == nil {
		t.Fatal("parseGetStorageInfoOutput() passed, but should have failed: ", info)
	}
}

func TestParseGetStorageInfoOutputSimpleHealthyNVMe(t *testing.T) {
	const out = `
	SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
	Percentage Used:                        25%
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
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
	Percentage Used:                        100%
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
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
		Device: SSD,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleHealthySSDPercentage(t *testing.T) {
	const out = `
	ATA Version is:   7
Device Statistics (GP Log 0x04)
Page  Offset Size        Value Flags Description
0x05  =====  =               =  ===  == Temperature Statistics (rev 1) ==
0x05  0x008  1              45  ---  Current Temperature
0x05  0x010  1               -  ---  Average Short Term Temperature
0x05  0x018  1               -  ---  Average Long Term Temperature
0x05  0x020  1              57  ---  Highest Temperature
0x05  0x028  1              26  ---  Lowest Temperature
0x05  0x030  1               -  ---  Highest Average Short Term Temperature
0x05  0x038  1               -  ---  Lowest Average Short Term Temperature
0x05  0x040  1               -  ---  Highest Average Long Term Temperature
0x05  0x048  1               -  ---  Lowest Average Long Term Temperature
0x05  0x050  4               0  ---  Time in Over-Temperature
0x05  0x058  1              95  ---  Specified Maximum Operating Temperature
0x05  0x060  4               0  ---  Time in Under-Temperature
0x05  0x068  1               0  ---  Specified Minimum Operating Temperature
0x07  =====  =               =  ===  == Solid State Device Statistics (rev 1) ==
0x07  0x008  1               2  N--  Percentage Used Endurance Indicator
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Device: SSD,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}

func TestParseGetStorageInfoOutputSimpleFailingSSDPercentage(t *testing.T) {
	const out = `
	ATA Version is:   7
Device Statistics (GP Log 0x04)
Page  Offset Size        Value Flags Description
0x05  =====  =               =  ===  == Temperature Statistics (rev 1) ==
0x05  0x008  1              45  ---  Current Temperature
0x05  0x010  1               -  ---  Average Short Term Temperature
0x05  0x018  1               -  ---  Average Long Term Temperature
0x05  0x020  1              57  ---  Highest Temperature
0x05  0x028  1              26  ---  Lowest Temperature
0x05  0x030  1               -  ---  Highest Average Short Term Temperature
0x05  0x038  1               -  ---  Lowest Average Short Term Temperature
0x05  0x040  1               -  ---  Highest Average Long Term Temperature
0x05  0x048  1               -  ---  Lowest Average Long Term Temperature
0x05  0x050  4               0  ---  Time in Over-Temperature
0x05  0x058  1              95  ---  Specified Maximum Operating Temperature
0x05  0x060  4               0  ---  Time in Under-Temperature
0x05  0x068  1               0  ---  Specified Minimum Operating Temperature
0x07  =====  =               =  ===  == Solid State Device Statistics (rev 1) ==
0x07  0x008  1              99  N--  Percentage Used Endurance Indicator
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Device: SSD,
		Status: Failing,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}
