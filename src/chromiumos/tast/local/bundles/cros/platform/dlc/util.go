// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides ways to interact with dlcservice daemon and utilities.
package dlc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

// Generic constants related to dlcservice.
const (
	ImageloaderManifestFile = "imageloader.json"
	PowerwashSafeDir        = "/mnt/stateful_partition/unencrypted/preserve"
	TmpDir                  = "/tmp"
)

// ImageloaderManifest holds the manifest information from rootfs imageloader.json file.
// Reference: https://chromium.googlesource.com/chromiumos/platform2.git/+/refs/heads/master/imageloader/manifest.md
type ImageloaderManifest struct {
	Size string `json:"size"`
}

// ReadImageloaderManifest returns the imageloader manifest read from rootfs.
func ReadImageloaderManifest(ctx context.Context, dlcID, dlcPackage string) (*ImageloaderManifest, error) {
	path := filepath.Join(ManifestDir, dlcID, dlcPackage, ImageloaderManifestFile)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Errorf("manifest doesn't exist for ID=%s Package=%s", dlcID, dlcPackage)
	}
	var manifest ImageloaderManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, errors.Wrap(err, "failed to read json")
	}
	return &manifest, nil
}

// RestartUpstartJob restarts the given job.
func RestartUpstartJob(ctx context.Context, job, serviceName string) error {
	// Restart job.
	if err := upstart.RestartJob(ctx, job); err != nil {
		return errors.Wrapf(err, "failed to restart %s", job)
	}

	// Wait for service to be ready.
	if bus, err := dbusutil.SystemBus(); err != nil {
		return errors.Wrap(err, "failed to connect to the message bus")
	} else if err := dbusutil.WaitForService(ctx, bus, serviceName); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", serviceName)
	}
	return nil
}
