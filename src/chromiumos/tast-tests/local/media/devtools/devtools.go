// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devtools provides common code for interacting with media Devtools.
package devtools

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mafredri/cdp/protocol/media"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// GetVideoDecoder waits for observer to produce a Player properties
// and parses it to figure out the video decoder name and if this is accelerated.
func GetVideoDecoder(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string) (isPlatform bool, name string, err error) {
	return getVideoCodecs(ctx, observer, url, true)
}

// GetVideoEncoder waits for observer to produce a Player properties
// and parses it to figure out the video encoder name and if this is accelerated.
func GetVideoEncoder(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string) (isPlatform bool, name string, err error) {
	return getVideoCodecs(ctx, observer, url, false)
}

func getVideoCodecs(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string, isDecoder bool) (isPlatform bool, name string, err error) {
	var hasPlatform, hasName bool

	platformProperty := "kIsPlatformVideoDecoder"
	nameProperty := "kVideoDecoderName"
	if !isDecoder {
		platformProperty = "kIsPlatformVideoEncoder"
		nameProperty = "kVideoEncoderName"
	}

	// We may not get all the properties on the first call to recv(), so poll for
	// a few seconds until we get them to account for that. This is due to how
	// Chrome DevTools sends out media player property updates.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		reply, err := observer.Recv()
		if err != nil {
			return err
		}

		for _, s := range reply.Properties {
			if s.Name == "kFrameUrl" && s.Value != url {
				return errors.New("failed to find the expected URL in Media DevTools")
			}

			if s.Name == platformProperty {
				hasPlatform = true
				isPlatform = s.Value == "true"
				testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
			} else if s.Name == nameProperty {
				hasName = true
				name = s.Value
				testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
			}

			if hasName && hasPlatform {
				break
			}
		}

		if !hasName && !hasPlatform {
			// Marshall reply.Properties to add it to the error log for debugging.
			var log string
			for _, s := range reply.Properties {
				log = fmt.Sprintf("%s, %s: %s", log, s.Name, s.Value)
			}
			return errors.Errorf("failed to find %s and %s in media DevTools Properties. Observed: %s", platformProperty, nameProperty, log)
		}
		if !hasName {
			return errors.Errorf("failed to find %s in media DevTools Properties", nameProperty)
		}
		if !hasPlatform {
			return errors.Errorf("failed to find %s in media DevTools Properties", platformProperty)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
	if err != nil {
		return false, "", err
	}
	return isPlatform, name, nil
}

// GetVideoPlaybackRoughness waits for and collects the video playback roughness
// as provided by Media DevTools. Simply put it's the RMSE of the actual
// playback times vs the ideal ones (1/fps). See go/chrome-video-roughness.
func GetVideoPlaybackRoughness(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string) (roughness float64, err error) {
	testing.ContextLog(ctx, "Waiting for DevTools video playback roughness meas")

	platformProperty := "kVideoPlaybackRoughness"

	type result struct {
		roughness float64
		err       error
	}

	c := make(chan result, 1)
	go func() {
		var hasValue bool
		err = testing.Poll(ctx, func(ctx context.Context) error {
			reply, err := observer.Recv()
			if err != nil {
				return errors.Wrap(err, "error receiving Media DevTools data")
			}

			for _, s := range reply.Properties {
				if s.Name == "kFrameUrl" && s.Value != url {
					return errors.New("failed to find the expected URL in Media DevTools")
				}

				if s.Name == platformProperty {
					hasValue = true
					if roughness, err = strconv.ParseFloat(s.Value, 64); err != nil {
						return err
					}
					testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
					break
				}
			}

			if !hasValue {
				return errors.Errorf("failed to find %s in media DevTools Properties", platformProperty)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second})

		c <- result{roughness, err}
	}()

	// Roughness takes 100s (or less, if the video finishes or is stopped early
	// but is longer than 20s at the time of writing). So we poll for a while,
	// e.g. 2 minutes (or whatever ctx timeout we have remaining, if shorter).
	const roughnessTimeout = 120 * time.Second
	shortCtx, cancel := context.WithTimeout(ctx, roughnessTimeout)
	defer cancel()
	select {
	case res := <-c:
		return res.roughness, res.err
	case <-shortCtx.Done():
		return 0, errors.New("Media DevTools timed out")
	}
}

// CheckHWDRMPipeline waits for observer to produce a Player properties
// and parses it to figure out if a the pipeline matches what we expect for HW
// DRM playback. That means the video is encrypted, we are using a HW video
// decoder and we are not using a video decrypting demuxer (which is a sign of
// L3 fallback in dev mode). It returns true if expectations are met for HW DRM.
func CheckHWDRMPipeline(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string) (isHWDRMPipeline bool, err error) {
	var hasPlatform, hasEncrypted, hasDemux, isPlatform, isVideoDecryptingDemuxer, isVideoEncrypted bool
	// We may not get all the properties on the first call to recv(), so poll for
	// a few seconds until we get them to account for that. This is due to how
	// Chrome DevTools sends out media player property updates.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		reply, err := observer.Recv()
		if err != nil {
			return err
		}

		for _, s := range reply.Properties {
			if s.Name == "kFrameUrl" && s.Value != url {
				return errors.New("failed to find the expected url in Media DevTools")
			}

			if s.Name == "kIsPlatformVideoDecoder" {
				hasPlatform = true
				isPlatform = s.Value == "true"
				testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
			} else if s.Name == "kIsVideoDecryptingDemuxerStream" {
				hasDemux = true
				isVideoDecryptingDemuxer = s.Value == "true"
				testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
			} else if s.Name == "kIsVideoEncrypted" {
				hasEncrypted = true
				isVideoEncrypted = s.Value == "true"
				testing.ContextLogf(ctx, "%s: %s", s.Name, s.Value)
			}

			if hasEncrypted && hasDemux && hasPlatform {
				break
			}
		}

		if !hasEncrypted && !hasDemux && !hasPlatform {
			// Marshall reply.Properties to add it to the error log for debugging.
			var log string
			for _, s := range reply.Properties {
				log = fmt.Sprintf("%s, %s: %s", log, s.Name, s.Value)
			}
			return errors.Errorf("failed to find kIsPlatformVideoDecoder, kVideoDecoderName, kIsVideoEncrypted and kIsVideoDecryptingDemuxerStream in media DevTools Properties. Observed: %s", log)
		}
		if !hasPlatform {
			return errors.New("failed to find kIsPlatformVideoDecoder in media DevTools Properties")
		}
		if !hasEncrypted {
			return errors.New("failed to find kIsVideoEncrypted in media DevTools Properties")
		}
		if !hasDemux {
			return errors.New("failed to find kIsVideoDecryptingDemuxerStream in media DevTools Properties")
		}
		if !isVideoEncrypted {
			return errors.New("video was not encrypted in HW DRM pipeline")
		}
		if !isPlatform {
			return errors.New("HW decoder was not used in HW DRM pipeline")
		}
		if isVideoDecryptingDemuxer {
			return errors.New("video decrypting dexmuer was used in HW DRM pipeline")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	if err != nil {
		return false, err
	}
	return true, nil
}
