// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Connector,
		Desc: "Checks the validity of display connector configurations",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func dumpModetestOnError(ctx context.Context, outDir string, hasError func() bool) {
	if !hasError() {
		return
	}
	file := filepath.Join(outDir, "modetest.txt")
	f, err := os.Create(file)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to create %s: %v", file, err)
		return
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "modetest", "-c")
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to run modetest: ", err)
	}
}

// Connector checks various attributes of the connectors settings via modetest.
func Connector(ctx context.Context, s *testing.State) {
	connectors, err := graphics.ModetestConnectors(ctx)
	if err != nil {
		s.Fatal("Failed to get connectors: ", err)
	}
	defer dumpModetestOnError(ctx, s.OutDir(), s.HasError)

	if err := checkUniqueEncoders(ctx, connectors); err != nil {
		s.Error("Failed to have check unique encoders: ", err)
	}
}

// checkUniqueEncoders checks if every connector can be assigned a unique encoder concurrently.
func checkUniqueEncoders(ctx context.Context, connectors []*graphics.Connector) error {
	encoderMap := make(map[int][]string)
	for _, connector := range connectors {
		// To simplify the code, we only cares about the connector with one encoder.
		if len(connector.Encoders) > 1 {
			testing.ContextLogf(ctx, "Connector %s has more than 1 encoders %v", connector.Name, connector.Encoders)
			continue
		}
		encoder := connector.Encoders[0]
		if con, ok := encoderMap[encoder]; !ok {
			encoderMap[encoder] = []string{connector.Name}
		} else {
			encoderMap[encoder] = append(con, connector.Name)
		}
	}

	var err error
	for encoder, connectors := range encoderMap {
		if len(connectors) > 1 {
			err = errors.Wrapf(err, "encoder %s is shared with multiple connector %v", encoder, connectors)
		}
	}
	return err
}
