// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
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
func (c *GmsCoreCacheService) Generate(ctx context.Context, req *empty.Empty) (*arcpb.GmsCoreCacheResponse, error) {
	const targetDir = "/tmp"

	// Boot ARC without existing GMS caches enabled to let GMS Core generate genuine caches.
	testing.ContextLog(ctx, "Starting ARC, with existing GMS caches disabled")

	cr, a, err := cache.OpenSession(ctx, cache.PackagesCopy, cache.GmsCoreDisabled, []string{}, targetDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generage GMS Core caches")
	}

	defer cr.Close(ctx)
	defer a.Close()

	if err := cache.CopyCaches(ctx, a, targetDir); err != nil {
		return nil, errors.Wrap(err, "failed to generage GMS Core caches")
	}

	response := arcpb.GmsCoreCacheResponse{
		PackagesCachePath: filepath.Join(targetDir, cache.PackagesCacheXML),
		GmsCoreCachePath:  filepath.Join(targetDir, cache.GmsCoreCacheArchive),
		GsfCachePath:      filepath.Join(targetDir, cache.GsfCache),
	}
	return &response, nil
}
