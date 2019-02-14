// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"io/ioutil"
	"strconv"

	"chromiumos/tast/local/bundles/cros/platform/fsinfo"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ReportDiskUsage,
		Desc:     "Reports available disk space in the root filesystem",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
	})
}

func ReportDiskUsage(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	// Report the production image size if it exists.
	const prodFile = "/root/bytes-rootfs-prod"
	if b, err := ioutil.ReadFile(prodFile); err == nil {
		if size, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64); err != nil {
			s.Errorf("Failed to parse %q from %v: %v", string(b), prodFile, err)
		} else {
			pv.Set(perf.Metric{
				Name:      "bytes_rootfs_prod",
				Unit:      "bytes",
				Direction: perf.SmallerIsBetter,
			}, float64(size))
		}
	}

	// Report the live size reported by df.
	if info, err := fsinfo.Get(ctx, "/"); err != nil {
		s.Error("Failed to get information about root filesystem: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "bytes_rootfs_test",
			Unit:      "bytes",
			Direction: perf.SmallerIsBetter,
		}, float64(info.Used))
	}
}
