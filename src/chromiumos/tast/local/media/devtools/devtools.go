// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devtools provides common code for interacting with media Devtools.
package devtools

import (
	"context"

	"github.com/mafredri/cdp/protocol/media"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// GetVideoDecoder waits for observer to produce a Player properties
// and parses it to figure out the video decoder name and if this is accelerated.
func GetVideoDecoder(ctx context.Context, observer media.PlayerPropertiesChangedClient, url string) (isPlatform bool, name string, err error) {
	reply, err := observer.Recv()
	if err != nil {
		return false, "", err
	}

	var hasPlatform, hasName bool
	for _, s := range reply.Properties {
		if s.Name == "kFrameUrl" && *s.Value != url {
			return false, "", errors.New("failed to find the expected url in Media DevTools")
		}

		if s.Name == "kIsPlatformVideoDecoder" {
			hasPlatform = true
			isPlatform = *s.Value == "true"
			testing.ContextLogf(ctx, "%s: %s", s.Name, *s.Value)
		}

		if s.Name == "kVideoDecoderName" {
			hasName = true
			name = *s.Value
			testing.ContextLogf(ctx, "%s: %s", s.Name, *s.Value)
		}
	}
	if !hasName && !hasPlatform {
		return false, "", errors.New("failed to find kIsPlatformVideoDecoder and kVideoDecoderName in media DevTools Properties")
	}
	if !hasName {
		return false, "", errors.New("failed to find kVideoDecoderName in media DevTools Properties")
	}
	if !hasPlatform {
		return false, "", errors.New("failed to find kIsPlatformVideoDecoder in media DevTools Properties")
	}

	return isPlatform, name, nil
}
