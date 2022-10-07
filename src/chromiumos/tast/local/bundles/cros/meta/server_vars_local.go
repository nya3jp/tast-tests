// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/common/servers"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServerVarsLocal,
		Desc:         "Demonstrate how to get server variable values in local tests",
		Contacts:     []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		BugComponent: "b:1034625",
	})
}

func ServerVarsLocal(ctx context.Context, s *testing.State) {
	servoHosts, err := verifyServerVars(servers.Servo)
	if err != nil {
		s.Error("Failed to verify servo variable: ", err)
	}
	testing.ContextLog(ctx, "Servo hosts Info: ", servoHosts)

	provisionServerHosts, err := verifyServerVars(servers.Provision)
	if err != nil {
		s.Error("Failed to verify provision server variable: ", err)
	}
	testing.ContextLog(ctx, "Provision server hosts info: ", provisionServerHosts)

	dutServerHosts, err := verifyServerVars(servers.DUT)
	if err != nil {
		s.Error("Failed to verify DUT server variable: ", err)
	}
	testing.ContextLog(ctx, "DUT server hosts Info: ", dutServerHosts)
}

func verifyServerVars(serverType servers.ServerType) (map[string]string, error) {
	hosts, err := servers.Servers(serverType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hosts")
	}
	for role, host := range hosts {
		h, err := servers.Server(serverType, role)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get host with role %s", role)
		}
		if h != host {
			return nil, errors.Errorf("got host %s for role %s; wanted %s", h, role, host)
		}
	}
	return hosts, nil
}
