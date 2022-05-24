// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cpuset

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
)

// Parse parses cpus file content from  /dev/cpuset/*/cpus and returns map of used CPUs.
// In case file could not be read, an error is returned.
func Parse(ctx context.Context, content string) (map[int]struct{}, error) {
	cpusInUse := make(map[int]struct{})
	for _, subset := range strings.Split(content, ",") {
		var fromCPU int
		var toCPU int

		if _, err := fmt.Sscanf(subset, "%d-%d", &fromCPU, &toCPU); err == nil {
			for i := fromCPU; i <= toCPU; i++ {
				cpusInUse[i] = struct{}{}
			}
			continue
		}
		if _, err := fmt.Sscanf(subset, "%d", &fromCPU); err == nil {
			cpusInUse[fromCPU] = struct{}{}
			continue
		}
		return nil, errors.Errorf("failed to parse cpus value %q", subset)
	}

	return cpusInUse, nil
}
