// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernelmeter

import (
	"testing"
)

func TestKernelWatermarks(t *testing.T) {
	const testInput = `Node 0, zone      DMA
  per-node stats
      nr_inactive_anon 5001
      ...
      blah blah blah
      ...
      nr_vmscan_immediate_reclaim 0
      nr_dirtied   49461
      nr_written   49173
                   40960
  pages free     3944
        min      66
        low      82
        high     98
        spanned  4095
        present  3999
        managed  3976
        protection: (0, 1881, 7909, 7909)
      nr_free_pages 3944
      nr_zone_inactive_anon 0
      nr_zone_active_anon 0

      ...

      nr_zspages   0
      nr_free_cma  0
  pagesets
    cpu: 0
              count: 0
              high:  0
              batch: 1
  vm stats threshold: 8
    cpu: 1
              count: 0
              high:  0
              batch: 1
  vm stats threshold: 8
    cpu: 2

    ...

  vm stats threshold: 8
    cpu: 7
              count: 0
              high:  0
              batch: 1
  vm stats threshold: 8
  node_unreclaimable:  0
  start_pfn:           1
Node 0, zone    DMA32
  pages free     481537
        min      8018
        low      10022
        high     12026
        spanned  1044480
        present  498055
        managed  481663
        protection: (0, 0, 6028, 6028)
      nr_free_pages 481537
      nr_zone_inactive_anon 0
      nr_zone_active_anon 0

      ...

      nr_free_cma  0
  pagesets
    cpu: 0
              count: 62
              high:  378
              batch: 63
  vm stats threshold: 40
    cpu: 1
              count: 0
              high:  378
              batch: 63
  vm stats threshold: 40
    cpu: 2
              count: 0
              high:  378
              batch: 63

   ...

Node 0, zone   Normal
  pages free     1006752
        min      25707
        low      32133
        high     38559
        spanned  1586176
        present  1586176
        managed  1544257
        protection: (0, 0, 0, 0)
      nr_free_pages 1006752
      nr_zone_inactive_anon 5001

      ...

Node 0, zone  Movable
  pages free     0
        min      0
        low      0
        high     0
        spanned  0
        present  0
        managed  0
        protection: (0, 0, 0, 0)
`
	const (
		pageSize             = MemSize(4096)
		expectedMin          = 33791 * pageSize
		expectedLow          = 42237 * pageSize
		expectedHigh         = 50683 * pageSize
		expectedTotalReserve = 72811 * pageSize
	)
	w, err := stringToWatermarks(testInput)
	if err != nil {
		t.Fatal("error in stringToWatermarks", err)
	}
	if w.min != expectedMin {
		t.Fatalf("min: got %v; want %v", w.min, expectedMin)
	}
	if w.low != expectedLow {
		t.Fatalf("low: got %v; want %v", w.low, expectedLow)
	}
	if w.high != expectedHigh {
		t.Fatalf("high: got %v; want %v", w.high, expectedHigh)
	}
	if w.totalReserve != expectedTotalReserve {
		t.Fatalf("totalReserve: got %v; want %v", w.totalReserve, expectedTotalReserve)
	}
}
