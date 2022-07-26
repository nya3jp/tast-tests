// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lorgnette

import (
	"context"
	"os"
	"path/filepath"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/errors"
)

// RunScan takes in the scan request and temporary directory needed to perform a scan operation
// Instantiates a new lorgnette instance and processes a complete scan job of a single image
// It returns the scan path of the scanned image
func RunScan(ctx context.Context, startScanRequest *lpb.StartScanRequest, tmpDir string) (string, error) {
	l, err := New(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to lorgnette")
	}

	startScanResponse, err := l.StartScan(ctx, startScanRequest)
	if err != nil {
		return "", errors.Wrap(err, "failed to call StartScan")
	}
	// Lorgnette was started automatically when we called StartScan, make sure to
	// close it when we exit.
	defer StopService(ctx)

	switch startScanResponse.State {
	case lpb.ScanState_SCAN_STATE_IN_PROGRESS:
		// Do nothing.
	case lpb.ScanState_SCAN_STATE_FAILED:
		return "", errors.Errorf("failed to start scan: %s", startScanResponse.FailureReason)
	default:
		return "", errors.Errorf("unexpected ScanState: %s", startScanResponse.State.String())
	}

	getNextImageRequest := &lpb.GetNextImageRequest{
		ScanUuid: startScanResponse.ScanUuid,
	}

	scanPath := filepath.Join(tmpDir, "scanned.png")
	scanFile, err := os.Create(scanPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open scan output file")
	}
	defer scanFile.Close()

	getNextImageResponse, err := l.GetNextImage(ctx, getNextImageRequest, scanFile.Fd())
	if err != nil {
		return "", errors.Wrap(err, "failed to call GetNextImage")
	}

	if !getNextImageResponse.Success {
		return "", errors.Errorf("failed to get next image: %s", getNextImageResponse.FailureReason)
	}

	if err = l.WaitForScanCompletion(ctx, startScanResponse.ScanUuid); err != nil {
		return "", errors.Wrap(err, "failed to wait for scan completion")
	}

	return scanPath, nil
}
