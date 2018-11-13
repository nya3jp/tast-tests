// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/compupdater"
)

const (
	componentName = "cros-termina" // The name of the Chrome component for the VM kernel and rootfs.
)

// LoadTerminaComponent loads the termina component that contains the VM kernel
// and rootfs. The path of the loaded component is returned. This is needed
// before running VMs.
func LoadTerminaComponent(ctx context.Context) (string, error) {
	updater, err := compupdater.New(ctx)
	if err != nil {
		return "", err
	}

	componentPath, err := updater.LoadComponent(ctx, componentName, compupdater.Mount)
	if err != nil {
		return "", errors.Wrapf(err, "mounting %q component failed", componentName)
	}

	return componentPath, nil
}
