// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"

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
			arcpb.RegisterTtsCacheServiceServer(srv, &TtsCacheService{s})
		},
	})
}

// TtsCacheService implements tast.cros.arc.TtsCacheService.
type TtsCacheService struct {
	s *testing.ServiceState
}

// Generate generates TTS cache.
func (c *TtsCacheService) Generate(ctx context.Context, request *empty.Empty) (res *arcpb.TtsCacheResponse, retErr error) {
	targetDir, err := ioutil.TempDir("", "tts_cache")
	if err != nil {
		return nil, errors.Wrap(err, "failed to created target dir for TTS cache")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(targetDir)
		}
	}()

	// Boot ARC without existing TTS caches enabled to let TTS generate cache.
	testing.ContextLog(ctx, "Starting ARC, with ArcEnableTTSCaching feature turned on")

	cr, a, err := cache.OpenSession(ctx, []string{"--enable-features=ArcEnableTTSCaching"}, targetDir)
	if err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage TTS cache")
	}

	defer cr.Close(ctx)
	defer a.Close(ctx)

	if err := cache.CopyTtsCache(ctx, a, targetDir); err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage GMS Core caches")
	}

	return &arcpb.TtsCacheResponse{
		TargetDir:         targetDir,
		TtsStateCacheName: cache.TTSStateCache,
	}, nil
}
