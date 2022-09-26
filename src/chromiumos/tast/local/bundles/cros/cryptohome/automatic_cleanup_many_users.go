// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanupManyUsers,
		Desc: "Test automatic disk cleanup",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for ChromeOS Storage
		},
		Params: []testing.Param{{
			Name:      "5_users",
			Val:       5,
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   3 * time.Minute,
		}, {
			Name:      "20_users",
			Val:       20,
			ExtraAttr: []string{"group:crosbolt", "crosbolt_nightly"},
			Timeout:   10 * time.Minute,
		}},
	})
}

func AutomaticCleanupManyUsers(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	userCount := s.Param().(int)

	const (
		homedirSize = 10 * cleanup.MiB

		userPrefix = "cleanup-user"
		password   = "1234"
	)

	// Start cryptohomed and wait for it to be available
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	defer daemonController.Restart(cleanupCtx, hwsec.CryptohomeDaemon)

	if err := cleanup.RunOnExistingUsers(ctx); err != nil {
		s.Fatal("Failed to perform initial cleanup: ", err)
	}

	pv := perf.NewValues()

	userCreationCtx, st := timing.Start(ctx, "user_creation")
	userCreationStart := time.Now()
	var fillFiles []string
	// Create user directories.
	for i := 1; i <= userCount; i++ {
		user := fmt.Sprintf("%s-%d", userPrefix, i)

		fillFile, err := cleanup.CreateFilledUserHomedir(userCreationCtx, user, password, "Cache", homedirSize)
		if err != nil {
			s.Fatal("Failed to create user with content: ", err)
		}
		defer cryptohome.RemoveVault(cleanupCtx, user)

		fillFiles = append(fillFiles, fillFile)
	}
	st.End()
	userCreationTime := time.Since(userCreationStart)

	pv.Set(perf.Metric{
		Name:      fmt.Sprintf("cryptohome_user_creation_%d", userCount),
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, float64(userCreationTime.Milliseconds()))

	// Unmount all users.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount users: ", err)
	}

	reader, err := syslog.NewReader(ctx, syslog.Program(syslog.Cryptohomed))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	automaticCleanupCtx, st := timing.Start(ctx, "cleanup")
	automaticCleanupStart := time.Now()
	if err := cleanup.ForceAutomaticCleanup(automaticCleanupCtx); err != nil {
		s.Fatal("Failed to run automatic cleanup: ", err)
	}
	st.End()
	automaticCleanupTime := time.Since(automaticCleanupStart)

	pv.Set(perf.Metric{
		Name:      fmt.Sprintf("cryptohome_start_and_cleanup_%d", userCount),
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, float64(automaticCleanupTime.Milliseconds()))

	re := regexp.MustCompile(`Disk cleanup took (\d+)ms.`)

	s.Log("Waiting for metric from cryptohomed")
	// Get cleanup duration from log.
	entry, err := reader.Wait(ctx, 30*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "Disk cleanup took")
	})
	if err != nil {
		s.Fatal("Cleanup not completed")
	}

	matches := re.FindStringSubmatch(entry.Content)
	if len(matches) < 2 {
		s.Fatalf("Failed to match regex %q in %q", re, entry.Content)
	}

	duration, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		s.Fatal("Failed to parse duration")
	}

	pv.Set(perf.Metric{
		Name:      fmt.Sprintf("cryptohome_initial_cleanup_%d", userCount),
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, duration)

	for _, fillFile := range fillFiles {
		if _, err := os.Stat(fillFile); err == nil {
			s.Error("fillFile still present")
		} else if !os.IsNotExist(err) {
			s.Fatalf("Failed to check if fill file %s exists: %v", fillFile, err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
