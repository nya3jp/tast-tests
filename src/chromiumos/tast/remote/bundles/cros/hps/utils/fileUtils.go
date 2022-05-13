// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains common api for those tests to use.
// fileutil contains functionality used by the HPS tast tests.
package utils

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	picFile       = "IMG_7451.jpg"
	noPersonFile  = "no-person-present.html"
	onePersonFile = "person-present.html"
	twoPeopleFile = "two-people-present.html"
)

// SetupDisplay sets up chart display for camerabox
func SetupDisplay(ctx context.Context, s *testing.State) ([]chart.NamePath, *chart.Chart, error) {
	archive := s.DataPath(hpsutil.PersonPresentPageArchiveFilename)
	filePaths, err := untarImages(ctx, archive)
	if err != nil {
		return []chart.NamePath{}, &chart.Chart{}, err
	}

	// Connecting to the other tablet that will render the picture.
	var chartAddr string
	if altAddr, ok := s.Var("tablet"); ok {
		chartAddr = altAddr
	}

	c, hostPaths, err := chart.New(ctx, s.DUT(), chartAddr, s.OutDir(), filePaths)
	if err != nil {
		return []chart.NamePath{}, &chart.Chart{}, errors.Wrap(err, "failed to send the files")
	}
	return hostPaths, c, nil
}

// untarImages is to untar tar file of images with different presence to the remote tablet
func untarImages(ctx context.Context, originPath string) ([]string, error) {
	dirPath := filepath.Dir(originPath)

	tarOut, err := testexec.CommandContext(ctx, "tar", "--strip-components=1", "-xvf", originPath, "-C", dirPath).Output()
	testing.ContextLog(ctx, "Extracting following files: ", string(tarOut))
	if err != nil {
		return nil, errors.Wrap(err, "failed to untar test artifacts")
	}

	picture := filepath.Join(dirPath, picFile)
	chartPaths := []string{
		filepath.Join(dirPath, noPersonFile),
		filepath.Join(dirPath, onePersonFile),
		filepath.Join(dirPath, twoPeopleFile)}

	filePaths := append(chartPaths, picture)

	return filePaths, nil
}
