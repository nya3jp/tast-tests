// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf provides utilities to build a JSON file that can be uploaded to
// Chrome Performance Dashboard (https://chromeperf.appspot.com/).
//
// Chrome Performance Dashboard docs can be found here:
// https://github.com/catapult-project/catapult/tree/master/dashboard
package perf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
)

var (
	// nameRe defines valid names (Name and Variant).
	nameRe = regexp.MustCompile("^[a-zA-Z0-9._-]{1,256}$")
	// unitRe defines valid units.
	unitRe = regexp.MustCompile("^[a-zA-Z0-9._-]{1,32}$")
)

// DefaultVariantName is the default variant name treated specially by the dashboard.
const DefaultVariantName = "summary"

// Direction indicates which direction of change (bigger or smaller) means improvement
// of a performance metric.
type Direction int

const (
	SmallerIsBetter Direction = iota
	BiggerIsBetter
)

// Metric defines the schema of a performance metric.
type Metric struct {
	// Name is the name of the chart this performance metric appears in.
	Name string

	// Variant is the name of this performance metric in a chart. If this is empty,
	// DefaultVariantName is used. It is treated specially by the dashboard.
	// Charts containing only one performance metric should stick with the default.
	Variant string

	// Unit is a unit name to describe values of this performance metric.
	Unit string

	// Direction indicates which direction of change (bigger or smaller) means improvement
	// of this performance metric.
	Direction Direction

	// Multiple specifies if this performance metric can contain multiple values at a time.
	Multiple bool
}

func (s *Metric) setDefaults() {
	if len(s.Variant) == 0 {
		s.Variant = DefaultVariantName
	}
}

// Values holds performance metric values.
type Values map[Metric][]float64

// Append appends performance metrics values. It can be called only for multi-valued
// performance metrics.
func (p *Values) Append(s Metric, vs ...float64) {
	s.setDefaults()
	if !s.Multiple {
		panic("Append must not be called for single-valued data series")
	}
	(*p)[s] = append((*p)[s], vs...)
	validate(s, (*p)[s])
}

// Set sets a performance metric value(s).
func (p *Values) Set(s Metric, vs ...float64) {
	s.setDefaults()
	(*p)[s] = vs
	validate(s, (*p)[s])
}

// traceData is a struct corresponding to a trace entry in Chrome Performance Dashboard JSON.
// See: https://github.com/catapult-project/catapult/blob/master/dashboard/docs/data-format.md
type traceData struct {
	Units                string    `json:"units"`
	ImprovementDirection string    `json:"improvement_direction"`
	Type                 string    `json:"type"`
	Value                float64   `json:"value,omitempty"`
	Values               []float64 `json:"values,omitempty"`
}

// Save saves performance metric values as a JSON file named "results-chart.json" in outDir.
// outDir should be the output directory path obtained from testing.State.
func (p *Values) Save(outDir string) error {
	charts := &map[string]*map[string]*traceData{}

	for s, vs := range *p {
		traces, ok := (*charts)[s.Name]
		if !ok {
			traces = &map[string]*traceData{}
			(*charts)[s.Name] = traces
		}

		var t traceData
		t.Units = s.Unit
		if s.Direction == BiggerIsBetter {
			t.ImprovementDirection = "up"
		} else {
			t.ImprovementDirection = "down"
		}
		if s.Multiple {
			t.Type = "list_of_scalar_values"
			t.Values = vs
		} else {
			t.Type = "scalar"
			t.Value = vs[0]
		}

		(*traces)[s.Variant] = &t
	}

	b, err := json.MarshalIndent(charts, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(outDir, "results-chart.json"), b, 0644)
}

func validate(s Metric, vs []float64) {
	if !nameRe.MatchString(s.Name) {
		panic(fmt.Sprintf("Metric has illegal Name: %v", s))
	}
	if !nameRe.MatchString(s.Variant) {
		panic(fmt.Sprintf("Metric has illegal Variant: %v", s))
	}
	if !unitRe.MatchString(s.Unit) {
		panic(fmt.Sprintf("Metric has illegal Unit: %v", s))
	}
	if !s.Multiple && len(vs) != 1 {
		panic(fmt.Sprintf("Metric requires single-valued: %v", s))
	}
}
