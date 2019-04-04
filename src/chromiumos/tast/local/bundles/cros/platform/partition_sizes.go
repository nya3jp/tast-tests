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

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PartitionSizes,
		Desc:     "Checks rootfs partition sizes",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},
	})
}

func PartitionSizes(ctx context.Context, s *testing.State) {
	// This mirrors get_fixed_dst_drive() in Autotest's client/bin/utils.py.
	out, err := testexec.CommandContext(ctx, "sh", "-c",
		". /usr/sbin/write_gpt.sh && . /usr/share/misc/chromeos-common.sh && "+
			"load_base_vars && get_fixed_dst_drive").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	baseDev := filepath.Base(strings.TrimSpace(string(out)))
	s.Log("Checking partitions on device ", baseDev)

	// If the device ends in a digit, a "p" appears before the partition number.
	// See concat_partition() in Autotest's client/bin/utils.py.
	partPrefix := baseDev
	if unicode.IsDigit(rune(baseDev[len(baseDev)-1])) {
		partPrefix += "p"
	}

	const gb = 1024 * 1024 * 1024
	validSizes := []int64{2 * gb, 4 * gb}

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
		} else {
			s.Errorf("%s is %d bytes; valid sizes are %v", partDev, bytes, validSizes)
		}
	}
}
