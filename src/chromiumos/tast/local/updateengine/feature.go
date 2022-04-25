// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides ways to interact with update_engine daemon and utilities.
package updateengine

import (
	"context"

	"chromiumos/tast/errors"
)

// Feature is the type of feature used internally during tast.
type Feature int64

// List of features update_engine currently supports.
const (
	ConsumerAutoUpdate Feature = iota
)

var featureDict = map[Feature]string{
	ConsumerAutoUpdate: "feature-consumer-auto-update",
}

func getFeatureMapping(ctx context.Context, feature Feature) (string, error) {
	if val, ok := featureDict[feature]; ok {
		return val, nil
	}
	return "", errors.New("get feature mapping: Invalid feature")
}
