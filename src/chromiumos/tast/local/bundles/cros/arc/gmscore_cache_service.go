// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/cache"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterGmsCoreCacheServiceServer(srv, &GmsCoreCacheService{s: s})
		},
	})
}

// GmsCoreCacheService implements tast.cros.arc.GmsCoreCacheService.
type GmsCoreCacheService struct {
	s *testing.ServiceState
}

// Generate generates GMS Core and GFS caches.
func (c *GmsCoreCacheService) Generate(ctx context.Context, request *arcpb.GmsCoreCacheRequest) (res *arcpb.GmsCoreCacheResponse, retErr error) {
	// Boot ARC without existing GMS caches enabled to let GMS Core generate genuine caches.
	testing.ContextLog(ctx, "Starting ARC, with existing GMS caches disabled")

	targetDir, err := ioutil.TempDir("", "gms_core_caches")
	if err != nil {
		return nil, errors.Wrap(err, "failed to created target dir for GMS Core caches")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(targetDir)
		}
	}()

	var packagesMode cache.PackagesMode
	if request.PackagesCacheEnabled {
		packagesMode = cache.PackagesCopy
	} else {
		packagesMode = cache.PackagesSkipCopy
	}

	var gmsCoreMode cache.GMSCoreMode
	if request.GmsCoreEnabled {
		gmsCoreMode = cache.GMSCoreEnabled
	} else {
		gmsCoreMode = cache.GMSCoreDisabled
	}

	cr, a, err := cache.OpenSession(ctx, packagesMode, gmsCoreMode, nil, targetDir)
	if err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage GMS Core caches")
	}

	defer cr.Close(ctx)
	defer a.Close()

	if err := cache.CopyCaches(ctx, a, targetDir); err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage GMS Core caches")
	}

	// This won't overwrite packages_cache.xml because
	// the TempFile function appends a random string
	// to the file name.
	temp, err := ioutil.TempFile(targetDir, cache.PackagesCacheXML)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temp file for generated %s", cache.PackagesCacheXML)
	}

	generatedPackagesCache := temp.Name()
	if err := a.PullFile(ctx, filepath.Join("/system/etc", cache.PackagesCacheXML), generatedPackagesCache); err != nil {
		return nil, errors.Wrapf(err, "could not pull %s from Android, this may mean that pre-generated packages cache was not installed when building the image: ", generatedPackagesCache)
	}

	response := arcpb.GmsCoreCacheResponse{
		TargetDir:                  targetDir,
		PackagesCacheName:          cache.PackagesCacheXML,
		GmsCoreCacheName:           cache.GMSCoreCacheArchive,
		GmsCoreManifestName:        cache.GMSCoreManifest,
		GsfCacheName:               cache.GSFCache,
		GeneratedPackagesCacheName: filepath.Base(generatedPackagesCache),
	}
	return &response, nil
}
