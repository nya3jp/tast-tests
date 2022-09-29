// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/bundles/cros/arc/cache"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
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

	args := []string{"--enable-features=ArcEnableTTSCacheSetup"}
	if !request.TtsCacheSetupEnabled {
		args = append(args, "--arc-disable-tts-cache")
	}

	// Boot ARC without existing TTS caches enabled to let TTS generate cache.
	testing.ContextLog(ctx, "Starting ARC with the following arguments: ", args)

	cr, a, err := cache.OpenSession(ctx, args, targetDir)
	if err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generate TTS cache")
	}

	defer cr.Close(ctx)
	defer a.Close(ctx)

	if err := cache.CopyTTSCache(ctx, a, targetDir); err != nil {
		os.RemoveAll(targetDir)
		return nil, errors.Wrap(err, "failed to generate TTS cache")
	}

	pregenCacheFileName := cache.PregeneratedTTSStateCache
	pregenTTSCacheFileSrc := filepath.Join("/system/etc", cache.TTSStateCache)
	pregenTTSCacheFileDst := filepath.Join(targetDir, pregenCacheFileName)
	if err := a.PullFile(ctx, pregenTTSCacheFileSrc, pregenTTSCacheFileDst); err != nil {
		testing.ContextLog(ctx, "Could not pull pregenerated TTS cache from Android, this may mean that the cache was not installed when building the image")
		pregenCacheFileName = ""
	}

	testing.ContextLog(ctx, "Waiting for TTS engine to be loaded")
	if err := a.WaitIntentHelper(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for ARC Intent Helper")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating test API connection")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		voices, err := a11y.Voices(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get voices")
		}

		for _, voice := range voices {
			if strings.HasPrefix(voice.Name, "Android") {
				return nil
			}
		}

		return errors.New("TTS engine is not loaded")
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return nil, errors.Wrap(err, "failed waiting for TTS engine to load")
	}

	// TODO(b/237255015): Remove check of the non-namespaced (ro.arc...) property
	// once Android is setting the properly-namespaced property (ro.vendor.arc...)
	const ttsCacheReadProp1 = "ro.vendor.arc.tts.initialized_from_cache"
	const ttsCacheReadProp2 = "ro.arc.tts.initialized_from_cache"
	propVal1, err1 := a.GetProp(ctx, ttsCacheReadProp1)
	propVal2, err2 := a.GetProp(ctx, ttsCacheReadProp2)
	if err1 != nil && err2 != nil {
		return nil, errors.Wrapf(err1, "failed to get prop %q and %q", ttsCacheReadProp1, ttsCacheReadProp2)
	}
	initializedFromCache := (err1 == nil && propVal1 == "1") || (err2 == nil && propVal2 == "1")

	return &arcpb.TTSCacheResponse{
		TargetDir:                     targetDir,
		TtsStateCacheName:             cache.TTSStateCache,
		PregeneratedTtsStateCacheName: pregenCacheFileName,
		EngineInitializedFromCache:    initializedFromCache,
	}, nil
}
