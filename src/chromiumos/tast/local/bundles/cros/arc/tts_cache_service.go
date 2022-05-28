// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/cache"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

const (
	initializationFromCacheTimeout = 10 * time.Second
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterTTSCacheServiceServer(srv, &TTSCacheService{s})
		},
	})
}

// TTSCacheService implements tast.cros.arc.TTSCacheService.
type TTSCacheService struct {
	s *testing.ServiceState
}

// Generate generates TTS cache.
func (c *TTSCacheService) Generate(ctx context.Context, request *arcpb.TTSCacheRequest) (res *arcpb.TTSCacheResponse, retErr error) {
	targetDir, err := ioutil.TempDir("", "tts_cache")
	if err != nil {
		return nil, errors.Wrap(err, "failed to created target dir for TTS cache")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(targetDir)
		}
	}()

	args := []string{"--enable-features=ArcEnableTTSCaching"}
	if !request.TtsCacheSetupEnabled {
		args = append(args, "--arc-disable-tts-cache")
	}

	// Boot ARC without existing TTS caches enabled to let TTS generate cache.
	testing.ContextLog(ctx, "Starting ARC with the following arguments: ", args)

	cr, a, err := cache.OpenSession(ctx, args, targetDir)
	if err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage TTS cache")
	}

	defer cr.Close(ctx)
	defer a.Close(ctx)

	if err := cache.CopyTTSCache(ctx, a, targetDir); err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generage TTS cache")
	}

	pregenCacheFileName := cache.PregeneratedTTSStateCache
	pregenTTSCacheFileSrc := filepath.Join("/system/etc", cache.TTSStateCache)
	pregenTTSCacheFileDst := filepath.Join(targetDir, pregenCacheFileName)
	if err := a.PullFile(ctx, pregenTTSCacheFileSrc, pregenTTSCacheFileDst); err != nil {
		testing.ContextLog(ctx, "Could not pull pregenerated TTS cache from Android, this may mean that the cache was not installed when building the image")
		pregenCacheFileName = ""
	}

	if request.TtsCacheSetupEnabled && request.WaitForCacheRead {
		const ttsCacheReadProp = "ro.arc.tts.initialized_from_cache"
		testing.ContextLog(ctx, "Waiting for TTS initialization from cache")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			propVal, getPropErr := a.GetProp(ctx, ttsCacheReadProp)
			if getPropErr != nil {
				return testing.PollBreak(errors.Wrapf(err, "failed to get prop %q", ttsCacheReadProp))
			}
			if propVal == "1" {
				return nil
			}
			return errors.Wrapf(err, "prop %q is not set yet", ttsCacheReadProp)
		}, &testing.PollOptions{Timeout: initializationFromCacheTimeout, Interval: time.Second}); err != nil {
			return nil, errors.Wrap(err, "failed to wait for cache read during TTS initialization")
		}
	}

	return &arcpb.TTSCacheResponse{
		TargetDir:                     targetDir,
		TtsStateCacheName:             cache.TTSStateCache,
		PregeneratedTtsStateCacheName: pregenCacheFileName,
	}, nil
}
