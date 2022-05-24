// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDiskErrorLines(t *testing.T) {
	splitLines := func(text string) []string {
		return strings.Split(text, "\n")
	}

	for _, tc := range []struct {
		name  string
		lines []string
		want  []string
	}{
		{
			name: "ATA errors",
			lines: splitLines(`[   49.135097] ata1.00: exception Emask 0x0 SAct 0x10000000 SErr 0x0 action 0x6 frozen
[   49.135112] ata1.00: failed command: READ FPDMA QUEUED
[   49.135126] ata1.00: cmd 60/40:e0:f0:f1:b5/00:00:00:00:00/40 tag 28 ncq dma 32768 in
[   49.135126]          res 40/00:00:00:00:00/00:00:00:00:00/00 Emask 0x4 (timeout)
[   49.135133] ata1.00: status: { DRDY }
[   49.135142] ata1: hard resetting link
[   49.445043] ata1: SATA link up 6.0 Gbps (SStatus 133 SControl 300)
[   49.448140] ata1.00: configured for UDMA/133
[   49.448173] ata1.00: device reported invalid CHS sector 0
[   49.448196] ata1: EH complete`),
			want: splitLines(`[   49.135097] ata1.00: exception Emask 0x0 SAct 0x10000000 SErr 0x0 action 0x6 frozen
[   49.135142] ata1: hard resetting link`),
		},
		{
			name: "SCSI errors",
			lines: splitLines(`[  241.378165] sd 0:0:0:0: [sda] 30031872 512-byte logical blocks: (15.4 GB/14.3 GiB)
[  241.378905] sd 0:0:0:0: [sda] Write Protect is off
[  241.378910] sd 0:0:0:0: [sda] Mode Sense: 43 00 00 00
[  241.379429] sd 0:0:0:0: [sda] Write cache: disabled, read cache: enabled, doesn't support DPO or FUA
[  241.414705] sd 0:0:0:0: [sda] Attached SCSI removable disk
[  241.614066] sd 0:0:0:0: [sda] tag#0 FAILED Result: hostbyte=DID_ERROR driverbyte=DRIVER_OK
[  241.614076] sd 0:0:0:0: [sda] tag#0 CDB: Read(10) 28 00 00 05 d0 80 00 00 08 00
[  241.614080] print_req_error: I/O error, dev sda, sector 381056
[  241.654058] sd 0:0:0:0: [sda] tag#0 FAILED Result: hostbyte=DID_ERROR driverbyte=DRIVER_OK
[  241.654068] sd 0:0:0:0: [sda] tag#0 CDB: Read(10) 28 00 00 01 50 48 00 00 30 00`),
			want: splitLines(`[  241.614066] sd 0:0:0:0: [sda] tag#0 FAILED Result: hostbyte=DID_ERROR driverbyte=DRIVER_OK
[  241.654058] sd 0:0:0:0: [sda] tag#0 FAILED Result: hostbyte=DID_ERROR driverbyte=DRIVER_OK`),
		},
		// Block device errors should be ignored because some of them are
		// produced for loopback devices, in which case errors are harmless.
		{
			name: "Block device errors (blk_update_request)",
			lines: splitLines(`[   16.076930] blk_update_request: I/O error, dev loop9, sector 0
[   16.076941] blk_update_request: I/O error, dev loop9, sector 0`),
			want: nil,
		},
		{
			name: "Block device errors (print_req_error)",
			lines: splitLines(`[  112.866869] print_req_error: I/O error, dev loop9, sector 0
[  112.866888] print_req_error: I/O error, dev loop9, sector 0
[  112.866893] Buffer I/O error on dev loop9, logical block 0, async page read`),
			want: nil,
		},
	} {
		got := diskErrorLines(tc.lines)
		if diff := cmp.Diff(got, tc.want); diff != "" {
			t.Errorf("%s: mismatch (-got +want):\n%s", tc.name, diff)
		}
	}
}
