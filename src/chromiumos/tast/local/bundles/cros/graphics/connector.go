// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graph"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// dpPlusModels are models that have DP++ support in BIOS.
// This list contains models that have wrong bios setting as of 2020/10.
// Please contact chromeos-gfx@ team to edit the list if new device exposes dp++ support on purpose.
var dpPlusModels = []string{
	"asuka",
	"cave",
	"caroline",
	"chell",
	// drallion family
	"drallion", "drallion360",
	// fizz family
	"fizz", "jax", "kench", "sion", "teemo",
	"guado",
	"karma",
	"lars",
	"lili",
	// nautilus family
	"nautilus", "nautiluslte",
	"nocturne",
	// nami family
	"akali", "akali360", "bard", "ekko", "pantheon", "sona", "syndra", "vayne",
	// octopus family
	"dorp",
	// puff family
	"duffy", "faffy", "kaisa", "noibat", "puff", "wyvern",
	// rammus family
	"leona", "shyvana",
	"rikku",
	"sentry",
	"soraka",
	// sarien family
	"arcada", "sarien",
	"tidus",
	// volteer family
	// TODO(b:190100059): Remove once VBT is fixed.
	"collis", "copano", "delbin", "drobit", "eldrid", "lillipup", "lindar", "volteer", "volteer2",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Connector,
		Desc: "Checks the validity of display connector configurations",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"no_qemu", "no_manatee"},
		Params: []testing.Param{
			{
				Name:              "",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(dpPlusModels...)),
			}, {
				Name:              "bad_bios",
				ExtraHardwareDeps: hwdep.D(hwdep.Model(dpPlusModels...)),
				ExtraAttr:         []string{"informational"},
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
	defer graphics.DumpModetestOnError(ctx, s.OutDir(), s.HasError)

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
