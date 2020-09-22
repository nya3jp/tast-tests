// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Connector,
		Desc: "Checks the validity of display connector configurations.",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Connector(ctx context.Context, s *testing.State) {
	connectors, err := graphics.ModetestConnectors(ctx)
	if err != nil {
		s.Fatal("Failed to get connectors: ", err)
	}
	// We require that no HDMI-A connectors are exposed.
	if err := checkHDMIA(ctx, connectors); err != nil {
		s.Error("Failed to verify DP++: ", err)
	}

}

// checkHDMIA checks if any connector is named HDMI-A-*
func checkHDMIA(ctx context.Context, connectors []*graphics.Connector) error {
	for _, connector := range connectors {
		if strings.HasPrefix(connector.Name, "HDMI-A-") {
			return errors.Errorf("found connector connected to HDMI-A: %v", connector)
		}
	}
	return nil
}
