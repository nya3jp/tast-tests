// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

const (
	defaultIterations = 10 // The number of boot iterations. Can be overridden by var "meta.ColdbootStartupPerf.iterations".
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColdbootStartupPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures startup metrics for Lacros after cold booting the system",
		Contacts:     []string{"hidehiko@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Vars:         []string{"meta.ColdbootStartupPerf.iterations"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name:              "rootfs_primary",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:              "omaha_primary",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "chrome",
		}},
	})
}

func coldbootStartupPerfOnce(ctx context.Context, s *testing.State, i, iterations int, username, password string) {
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
		"-var=lacros.StartupPerf.iterations=1",
		"-var=lacros.StartupPerf.credentials=" + username + ":" + password,
		"-var=skipInitialLogin=true",
	}

	if err := execStartupPerf(ctx, s, flags); err != nil {
		s.Fatal("Failed to run tast: ", err)
	}
}

func ColdbootStartupPerf(ctx context.Context, s *testing.State) {
	iterations := defaultIterations
	if iter, ok := s.Var("meta.ColdbootStartupPerf.iterations"); ok {
		i, err := strconv.Atoi(iter)
		if err != nil {
			s.Fatal("Invalid meta.ColdbootStartupPerf.iterations value: ", iter)
		}
		iterations = i
	}

	username, password, err := pickRandomCreds(s.RequiredVar("ui.gaiaPoolDefault"))
	if err != nil {
		s.Fatal("Failed to get Gaia credentials: ", err)
	}

	// Run the startup local test once to set up the environment.
	flags := []string{
		"-var=lacros.StartupPerf.iterations=1",
		"-var=lacros.StartupPerf.credentials=" + username + ":" + password,
		"-var=skipRegularLogin=true",
	}

	if err := execStartupPerf(ctx, s, flags); err != nil {
		s.Fatal("Failed to run tast: ", err)
	}

	for i := 0; i < iterations; i++ {
		// Run the startup local test to actually get the metrics.
		coldbootStartupPerfOnce(ctx, s, i, iterations, username, password)
	}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// execStartupPerf executes specific variants of the lacros.StartupPerf local test. The variant
// chosen to run follows the same variant determined by suffix of the caller e.g.
// meta.ColdbootStartupPerf.rootfs_primary calls lacros.StartupPerf.rootfs_primary.
func execStartupPerf(ctx context.Context, s *testing.State, flags []string) error {
	variantName := strings.Split(s.TestName(), ".")[2]
	if _, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"lacros.StartupPerf." + variantName}); err != nil {
		errors.Errorf("failed to run lacros.StartupPerf tast: %s", err)
	}
	return nil
}

// pickRandomCreds picks a random user and password from a list of credentials. Inspired by
// remote_tests/ui/chrome_service_grpc.go.
//
// creds is a string containing multiple credentials separated by newlines:
//  user1:pass1
//  user2:pass2
//  user3:pass3
//  ...
func pickRandomCreds(creds string) (string, string, error) {
	// Pick a random line
	lines := strings.Split(creds, "\n")
	randomIndex := random.Intn(len(lines))
	line := lines[randomIndex]

	// Extract user and password from the concatenated string
	line = strings.TrimSpace(line)
	userNamePassword := strings.SplitN(line, ":", 2)
	if len(userNamePassword) != 2 {
		return "", "", errors.Errorf("failed to parse credential list: line %d: does not contain a colon", randomIndex+1)
	}
	return userNamePassword[0], userNamePassword[1], nil
}
