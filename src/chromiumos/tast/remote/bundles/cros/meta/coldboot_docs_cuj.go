// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColdbootDocsCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Capture Google Docs CUJ metrics after cold booting the system",
		Contacts:     []string{"hidehiko@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Vars:         []string{"meta.ColdbootDocsCUJ.iterations"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "reverse",
			Val:  true,
		}},
	})
}

func ColdbootDocsCUJ(ctx context.Context, s *testing.State) {
	const (
		// The number of iterations. In order to collect meaningful average and data variability,
		// the default value is defined large enough as "10". Can be overridden by var
		// "meta.ColdbootDocsCUJ.iterations".
		defaultIterations = 10
	)

	iterations := defaultIterations
	if iter, ok := s.Var("meta.ColdbootDocsCUJ.iterations"); ok {
		i, err := strconv.Atoi(iter)
		if err != nil {
			s.Fatal("Invalid meta.ColdbootDocsCUJ.iterations value: ", iter)
		}
		iterations = i
	}

	for i := 0; i < iterations; i++ {
		// Run the Docs CUJ local test to get the metrics.
		coldbootDocsCUJOnce(ctx, s, i, iterations)
	}
}

func coldbootDocsCUJOnce(ctx context.Context, s *testing.State, i, iterations int) {
	isReverse := s.Param().(bool)

	s.Logf("Running iteration %d/%d", i+1, iterations)
	d := s.DUT()

	// TODO(https://crbug.com/1346752): Find a way to get rid of CTRL+D trick "OS verification is
	// OFF" every time it boots.
	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Need to reconnect to the gRPC server after rebooting DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	resultsDir := filepath.Join(s.OutDir(), fmt.Sprintf("subtest_results_%d", i))

	flags := []string{
		"-resultsdir=" + resultsDir,
		"-var=lacros.DocsCUJ.iterations=1",
	}

	var variantName string
	if isReverse {
		variantName = ".reverse"
	}
	if _, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"lacros.DocsCUJ" + variantName}); err != nil {
		s.Fatal("Failed to run tast: ", err)
	}
}
