// Copyright 2022 The ChromiumOS Authors.
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

var (
	defaultIterations = 10 // The number of boot iterations. Can be overridden by var "meta.ColdbootStartupPerf.iterations".

	username string

	password string
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColdbootStartupPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures startup metrics for Lacros after cold booting the system",
		Contacts:     []string{"tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Vars:         []string{"meta.ColdbootStartupPerf.iterations"},
	})
}

func coldbootStartupPerfOnce(ctx context.Context, s *testing.State, i, iterations int) {
	// TODO(tvignatti): Find a way to get rid of CTRL+D trick "OS verification is OFF" every time
	// it boots.

	s.Logf("Running iteration %d/%d", i+1, iterations)
	d := s.DUT()

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

	s.Run(ctx, "BrowserLaunch", func(ctx context.Context, s *testing.State) {
		resultsDir := filepath.Join(s.OutDir(), fmt.Sprintf("subtest_results_%d", i))
		flags := []string{
			"-resultsdir=" + resultsDir,
			"-var=skipInitialLogin=true",
			"-var=username=" + username,
			"-var=password=" + password,
		}

		// TODO(tvignatti): omaha_primary seems the culprit for the slowness. So we need to run
		// that instead.
		_, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"lacros.StartupPerf.rootfs_primary"})
		if err != nil {
			s.Fatal("Failed to run tast: ", err)
		}
	})
}

func ColdbootStartupPerf(ctx context.Context, s *testing.State) {
	iterations := defaultIterations
	if iter, ok := s.Var("meta.ColdbootStartupPerf.iterations"); ok {
		if i, err := strconv.Atoi(iter); err == nil {
			iterations = i
		} else {
			// User might want to override the default value of iterations but passed a malformed
			// value. Fail the test to inform the user.
			s.Fatal("Invalid meta.ColdbootStartupPerf.iterations value: ", iter)
		}
	}

	user, pass, err := pickRandomCreds(s.RequiredVar("ui.gaiaPoolDefault"))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	username = user
	password = pass

	flags := []string{
		"-var=skipRegularLogin=true",
		"-var=username=" + username,
		"-var=password=" + password,
	}

	// TODO(tvignatti): omaha_primary seems the culprit for the slowness. So we need to run
	// that instead.
	_, _, err = tastrun.Exec(ctx, s, "run", flags, []string{"lacros.StartupPerf.rootfs_primary"})
	if err != nil {
		s.Fatal("Failed to run tast: ", err)
	}

	for i := 0; i < iterations; i++ {
		// Run the startup test once.
		coldbootStartupPerfOnce(ctx, s, i, iterations)
	}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

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
