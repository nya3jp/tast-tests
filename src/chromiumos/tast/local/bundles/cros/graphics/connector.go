// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graph"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// dpPlusModels are models that have DPPlus support in BIOS.
var dpPlusModels = []string{
	"asuka",
	"chell",
	"cave",
	"caroline",
	// drallion family
	"drallion", "drallion360",
	// TODO(b:16119031): Unmark dedede once it is fixed.
	// dedede family
	"drawlat", "drawman",
	// fizz family
	"teemo", "jax",
	"guado",
	"karma",
	"lars",
	"nautilus",
	"nocturne",
	// nami family
	"vayne", "sona", "pantheon", "ekko", "bard", "akali", "akali360", "syndra",
	// octopus family
	"dorp",
	// puff family
	"wyvern", "puff", "nooibat", "kaisa", "faffy", "duffy",
	// rammus family
	"shyvana", "leona",
	"rikku",
	"sentry",
	"soraka",
	// sarien family
	"sarien", "arcada",
	"tidus",
	// volteer family
	"volteer", "volteer2", "delbin",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Connector,
		Desc: "Checks the validity of display connector configurations",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(dpPlusModels...)),
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(hwdep.Model(dpPlusModels...)),
			},
		},
	})
}

// Connector checks various attributes of the connectors settings via modetest.
func Connector(ctx context.Context, s *testing.State) {
	connectors, err := graphics.ModetestConnectors(ctx)
	if err != nil {
		s.Fatal("Failed to get connectors: ", err)
	}
	defer dumpModetestOnError(ctx, s)

	if err := checkUniqueEncoders(ctx, connectors); err != nil {
		s.Error("Failed to have check unique encoders: ", err)
	}
}

// checkUniqueEncoders checks if every connector can be assigned a unique encoder concurrently.
func checkUniqueEncoders(ctx context.Context, connectors []*graphics.Connector) error {
	g := graph.NewBipartite()
	for _, connector := range connectors {
		for _, encoder := range connector.Encoders {
			g.AddEdge(connector.Cid, encoder)
		}
	}

	maxMatch := g.MaxMatching()
	if maxMatch != len(connectors) {
		return errors.Errorf("not all connector has a unqiue encoder matching (expect %d but got %d)", len(connectors), maxMatch)
	}
	return nil
}

// dumpModetestOnError dumps the output of modetest to a file if the test failed.
func dumpModetestOnError(ctx context.Context, s *testing.State) {
	if !s.HasError() {
		return
	}
	file := filepath.Join(s.OutDir(), "modetest.txt")
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
