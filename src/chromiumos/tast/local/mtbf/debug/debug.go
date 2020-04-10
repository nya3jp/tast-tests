// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debug

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

// Screenshots provides sceenshot capture functions
type Screenshots struct {
	ssch chan int // screenshot channel
}

// Start saves screenshots with given interval.
func (du *Screenshots) Start(ctx context.Context, interval time.Duration) error {
	if du.ssch != nil {
		return errors.New("screenshots is in progress")
	}

	dir, err := getDir(ctx)
	if err != nil {
		return err
	}
	du.ssch = make(chan int)
	go func() {
		faillog.SaveScreenshot(ctx, dir, "_"+time.Now().Format("20060102-150405"))

		for {
			select {
			case <-time.After(interval * time.Second):
				faillog.SaveScreenshot(ctx, dir, "_"+time.Now().Format("20060102-150405"))
			case <-du.ssch:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// TakeScreenshot saves one screenshot.
func TakeScreenshot(ctx context.Context) error {
	dir, err := getDir(ctx)
	if err != nil {
		return err
	}
	faillog.SaveScreenshot(ctx, dir, "_"+time.Now().Format("20060102-150405"))
	return nil
}

//Stop stops screenshots
func (du *Screenshots) Stop() {
	if du.ssch != nil {
		close(du.ssch)
		du.ssch = nil
	}
}

func getDir(ctx context.Context) (string, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	// If test setup failed, then the output dir may not exist.
	if !ok || outDir == "" {
		return "", errors.New("outDir is not set")
	}
	if _, err := os.Stat(outDir); err != nil {
		return "", errors.New("outDir does not exist")
	}

	dir := filepath.Join(outDir, "screenshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", errors.New("cannot make screenshots directory")
	}
	return dir, nil
}
