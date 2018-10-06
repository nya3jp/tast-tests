// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	"chromiumos/tast/local/dbusutil"
)

const (
	componentName = "cros-termina" // The name of the Chrome component for the VM kernel and rootfs.
)

// LoadTerminaComponent loads the termina component that contains the VM kernel
// and rootfs. The path of the loaded component is returned. This is needed
// before running VMs.
func LoadTerminaComponent(ctx context.Context) (string, error) {
	_, updater, err := dbusutil.Connect(ctx, componentUpdaterName, componentUpdaterPath)
	if err != nil {
		return "", err
	}

	var componentPath string
	err = updater.CallWithContext(ctx, componentUpdaterInterface+".LoadComponent", 0, componentName).Store(&componentPath)
	if err != nil {
		return "", fmt.Errorf("mounting %q component failed: %v", componentName, err)
	}

	return componentPath, nil
}
