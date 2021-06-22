// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/rialto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PartitionSizes,
		Desc:     "Checks rootfs partition sizes",
		Contacts: []string{"chromeos-systems@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

func PartitionSizes(ctx context.Context, s *testing.State) {
	// Get the internal disk device's name, e.g. "/dev/sda".
	const script = `
set -e
. /usr/sbin/write_gpt.sh
. /usr/share/misc/chromeos-common.sh
load_base_vars
get_fixed_dst_drive`
	out, err := testexec.CommandContext(ctx, "sh", "-c", strings.TrimSpace(script)).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	baseDev := filepath.Base(strings.TrimSpace(string(out)))
	if baseDev == "." { // filepath.Base("") returns "."
		baseDev = "sda"
		s.Logf("Got empty device; defaulting to %q (running in VM?)", baseDev)
	}
	s.Log("Checking partitions on device ", baseDev)

	// If the device ends in a digit, a "p" appears before the partition number.
	partPrefix := baseDev
	if unicode.IsDigit(rune(baseDev[len(baseDev)-1])) {
		partPrefix += "p"
	}

	const gb = 1024 * 1024 * 1024
	validSizes := []int64{
		2 * gb,
		4 * gb,
	}
	// Rialto devices may use 1 GB partitions.
	if isRialto, err := rialto.IsRialto(); err != nil {
		s.Error("Failed to check if device is rialto: ", err)
	} else if isRialto {
		validSizes = append(validSizes, 1*gb)
	}

	for _, partNum := range []int{3, 5} {
		partDev := partPrefix + strconv.Itoa(partNum)

		// This file contains the partition size in 512-byte sectors.
		// See https://patchwork.kernel.org/patch/7922301/ .
		sizePath := fmt.Sprintf("/sys/block/%s/%s/size", baseDev, partDev)
		out, err := ioutil.ReadFile(sizePath)
		if err != nil {
			s.Errorf("Failed to get %s size: %v", partDev, err)
			continue
		}
		sectors, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			s.Errorf("Failed to parse %q from %s: %v", out, sizePath, err)
			continue
		}
		bytes := sectors * 512

		valid := false
		for _, vs := range validSizes {
			if bytes == vs {
				valid = true
				break
			}
		}
		if valid {
			s.Logf("%s is %d bytes", partDev, bytes)
		} else if partNum == 5 && bytes < 10*1024*1024 {
			// Test images tend to use a stub ROOT-B, so allow very small ones.
			s.Logf("%s is %d bytes; ignoring stub partition", partDev, bytes)
		} else {
			s.Errorf("%s is %d bytes; valid sizes are %v", partDev, bytes, validSizes)
		}
	}
}
