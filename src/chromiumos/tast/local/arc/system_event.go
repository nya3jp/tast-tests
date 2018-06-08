// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

// WaitArcIntentHelper waits for ArcIntentHelper to get ready.
func WaitArcIntentHelper(ctx context.Context) error {
	newCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return waitSystemEvent(newCtx, "ArcIntentHelperService:ready")
}

func waitSystemEvent(ctx context.Context, name string) error {
	cmd := CommandContext(ctx, "logcat", "-b", "events", "*:S", "arc_system_event")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	testing.ContextLogf(ctx, "Waiting for ARC system event %v", name)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		testing.ContextLog(ctx, line)
		if strings.HasSuffix(line, " "+name) {
			return nil
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("ARC system event %v never seen", name)
}
