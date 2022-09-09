// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import "chromiumos/tast/errors"

// StatsMap is for the Stats field of PeerConnection.
type StatsMap map[StatsKey]Statistic

// StatsTimeline contains timeline data for a statistic.
type StatsTimeline []interface{}

// StatsIndexByAttribute maps StatsKey.Attribute to a StatsTimeline.
type StatsIndexByAttribute map[string]StatsTimeline

// StatsIndexByStatsID maps StatsKey.ID to a StatsIndexByAttribute.
type StatsIndexByStatsID map[string]StatsIndexByAttribute

// StatsIndexByStatsType maps Statistic.StatsType to a StatsIndexByStatsID.
type StatsIndexByStatsType map[string]StatsIndexByStatsID

// BuildIndex returns a StatsIndexByStatsType.
func (stats StatsMap) BuildIndex() StatsIndexByStatsType {
	byType := make(StatsIndexByStatsType)
	for key, stat := range stats {
		byID, ok := byType[stat.StatsType]
		if !ok {
			byID = make(StatsIndexByStatsID)
			byType[stat.StatsType] = byID
		}
		byAttribute, ok := byID[key.ID]
		if !ok {
			byAttribute = make(StatsIndexByAttribute)
			byID[key.ID] = byAttribute
		}
		byAttribute[key.Attribute] = StatsTimeline(stat.Values)
	}
	return byType
}

// Collapse verifies that all values in the timeline are equal, and returns the
// value. Use Collapse to read attributes that you don't expect to vary over time.
func (timeline StatsTimeline) Collapse() (interface{}, error) {
	if len(timeline) == 0 {
		return nil, errors.New("timeline is empty")
	}
	firstValue := timeline[0]
	for i := 1; i < len(timeline); i++ {
		if timeline[i] != firstValue {
			return nil, errors.Errorf("expected values all equal, got: %v", timeline)
		}
	}
	return firstValue, nil
}
