// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements implements interfaces and generic structs for power measurement

package power

import (
	"time"

	"chromiumos/tast/remote/firmware/suspend"
)

// Results is an interface for power measurement results
type Results interface {
	GetMean(key string) (float32, bool)
}

// Interface is an interface that provides power measurements
type Interface interface {
	Measure(targetState suspend.State, duration time.Duration) (Results, error)
}

// ResultsGeneric provides a simple wrapper around maps of measurements
type ResultsGeneric struct {
	means map[string]float32
}

// NewResultsGeneric creates a new ResultsGeneric
func NewResultsGeneric() ResultsGeneric {
	return ResultsGeneric{
		means: make(map[string]float32),
	}
}

// AddMeans copies means from an existing map
func (m *ResultsGeneric) AddMeans(means map[string]float32) {
	for key, value := range means {
		m.means[key] = value
	}
}

// GetMean returns the mean for a measurement by a key
func (m *ResultsGeneric) GetMean(key string) (float32, bool) {
	value, present := m.means[key]
	return value, present
}
