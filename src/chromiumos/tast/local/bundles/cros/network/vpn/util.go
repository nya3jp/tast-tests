// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// VerifyVPNProfile verifies a VPN service with certain GUID exists in shill, and can be connected if |verifyConnect| set to true.
func VerifyVPNProfile(ctx context.Context, m *shill.Manager, serviceGUID string, verifyConnect bool) error {
	testing.ContextLog(ctx, "Trying to find service with guid ", serviceGUID)

	findServiceProps := make(map[string]interface{})
	findServiceProps["GUID"] = serviceGUID
	findServiceProps["Type"] = "vpn"
	service, err := m.WaitForServiceProperties(ctx, findServiceProps, 5*time.Second)
	if err != nil {
		testing.ContextLog(ctx, "Cannot find service matching guid ", serviceGUID)
		return err
	}
	testing.ContextLogf(ctx, "Found service %v matching guid %s", service, serviceGUID)

	if !verifyConnect {
		return nil
	}

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)
	if err = service.Connect(ctx); err != nil {
		return errors.Wrapf(err, "failed to connect the service %v", service)
	}
	defer func() {
		if err = service.Disconnect(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to disconnect service ", service)
		}
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	state, err := pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, append(shillconst.ServiceConnectedStates, shillconst.ServiceStateFailure))
	if err != nil {
		return err
	}

	if state == shillconst.ServiceStateFailure {
		return errors.Errorf("service %v became failure state", service)
	}
	return nil
}
