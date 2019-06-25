// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
)

// ParseCPUs parses cpus file content and returns map of used CPUs.
// In case file could not be read, an error is returned.
func ParseCPUs(ctx context.Context, content string) (map[int]struct{}, error) {
	cpusInUse := make(map[int]struct{})
	for _, subset := range strings.Split(content, ",") {
		var fromCPU int
		var toCPU int
		n, err := fmt.Sscanf(subset, "%d-%d", &fromCPU, &toCPU)
		if err == nil && n == 2 {
			for i := fromCPU; i <= toCPU; i++ {
				cpusInUse[i] = struct{}{}
			}
			continue
		}
		n, err = fmt.Sscanf(subset, "%d", &fromCPU)
		if err == nil && n == 1 {
			cpusInUse[fromCPU] = struct{}{}
			continue
		}
		return nil, errors.New(fmt.Sprintf("Failed to parse token %s of %s", subset, content))
	}

	return cpusInUse, nil
}
