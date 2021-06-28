// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lorgnette

import (
	"context"
	"os"
	"path/filepath"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/testing"
)

// RunScan takes in the scan request and temporary directory needed to perform a scan operation
// Instantiates a new lorgnette instance and processes a complete scan job of a single image
// It returns the scan path of the scanned image
func RunScan(ctx context.Context, s *testing.State, startScanRequest *lpb.StartScanRequest, tmpDir string) string {
	l, err := New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to lorgnette: ", err)
	}

	s.Log("Starting scan")
	startScanResponse, err := l.StartScan(ctx, startScanRequest)
	if err != nil {
		s.Fatal("Failed to call StartScan: ", err)
	}
	// Lorgnette was started automatically when we called StartScan, make sure to
	// close it when we exit.
	defer StopService(ctx)

	switch startScanResponse.State {
	case lpb.ScanState_SCAN_STATE_IN_PROGRESS:
		// Do nothing.
	case lpb.ScanState_SCAN_STATE_FAILED:
		s.Fatal("Failed to start scan: ", startScanResponse.FailureReason)
	default:
		s.Fatal("Unexpected ScanState: ", startScanResponse.State.String())
	}

	getNextImageRequest := &lpb.GetNextImageRequest{
		ScanUuid: startScanResponse.ScanUuid,
	}

	scanPath := filepath.Join(tmpDir, "scanned.png")
	scanFile, err := os.Create(scanPath)
	if err != nil {
		s.Fatal("Failed to open scan output file: ", err)
	}

	s.Log("Getting next image")
	getNextImageResponse, err := l.GetNextImage(ctx, getNextImageRequest, scanFile.Fd())
	if err != nil {
		s.Fatal("Failed to call GetNextImage: ", err)
	}

	if !getNextImageResponse.Success {
		s.Fatal("Failed to get next image: ", getNextImageResponse.FailureReason)
	}

	s.Log("Waiting for completion signal")
	if err = l.WaitForScanCompletion(ctx, startScanResponse.ScanUuid); err != nil {
		s.Fatal("Failed to wait for scan completion: ", err)
	}

	return scanPath
}
