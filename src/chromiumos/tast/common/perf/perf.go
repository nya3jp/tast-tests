// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf provides utilities to build a JSON file that can be uploaded to
// Chrome Performance Dashboard (https://chromeperf.appspot.com/).
//
// Measurements processed by this package are stored in
// tests/<test-name>/results-chart.json in the Tast results dir.  The data is
// typically read by the Autotest TKO parser. In order to have metrics
// uploaded, they have to be listed here:
// src/third_party/autotest/files/tko/perf_upload/perf_dashboard_config.json
//
// Chrome Performance Dashboard docs can be found here:
// https://github.com/catapult-project/catapult/tree/master/dashboard
//
// Usage example:
//
//  pv := perf.NewValues()
//  pv.Set(perf.Metric{
//      Name:       "mytest_important_quantity"
//      Unit:       "gizmos"
//      Direction:  perf.BiggerIsBetter
//  }, 42)
//  if err := pv.Save(s.OutDir()); err != nil {
//      s.Error("Failed saving perf data: ", err)
//  }
//
// Remote usage example
//
// Protocol buffer definition:
//  import "values.proto";
//  service ExampleService {
//      rpc Method (google.protobuf.Empty)
//          returns (tast.common.perf.perfpb.Values) {}
//  }
// In order to "import values.proto", add a -I argument pointing at
// src/chromiumos/tast/common/perf/perfpb/ to the protoc command in your
// service's gen.go file. See src/chromiumos/tast/services/cros/arc/gen.go
// for an example.
//
// Service:
//  import "chromiumos/tast/common/perf"
//  import "chromiumos/tast/common/perf/perfpb"
//  func (s *ExampleService) Method() (*perfpb.Values, error) {
//      p := perf.NewValues()
//      ... // Do some computation that generates perf values in p.
//      return p.Proto(), nil
//  }
//
// Test:
//  import "chromiumos/tast/common/perf"
//  func TestMethod(ctx context.Context, s *testing.State) {
//      ... // Set up gRPC, ExampleServiceClient.
//      res, err := service.Method()
//      if err != nil {
//          s.Fatal("RPC failed: ", err)
//      }
//      if err := perf.NewValuesFromProto(res).Save(s.OutDir()); err != nil {
//          s.Fatal("Failed to save perf results: ", err)
//      }
//  }
package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/google/uuid"

	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/errors"
)

var (
	// nameRe defines valid names (Name and Variant).
	nameRe = regexp.MustCompile("^[a-zA-Z0-9._-]{1,256}$")
	// unitRe defines valid units.
	unitRe = regexp.MustCompile("^[a-zA-Z0-9._-]{1,32}$")
)

// DefaultVariantName is the default variant name treated specially by the dashboard.
const DefaultVariantName = "summary"

// genGUID generates a guid for diagnostic structs.
func genGUID(ctx context.Context) (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// Overridable function pointer for tests.
var runGenGUID = genGUID

// Direction indicates which direction of change (bigger or smaller) means improvement
// of a performance metric.
type Direction int

const (
	// SmallerIsBetter means the performance metric is considered improved when it decreases.
	SmallerIsBetter Direction = iota

	// BiggerIsBetter means the performance metric is considered improved when it increases.
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

// Maps a Metric unit type to a histogram unit type. TODO(stevenjb) Investigate
var supportedUnits = map[string]string{
	"bytes": "sizeInBytes",
	"J":     "J",
	"W":     "W",
	"count": "count",
	"ms":    "ms",
	"n%":    "n%",
	"sigma": "sigma",
	"tsMs":  "tsMs",
}

func (s *Metric) histogramUnit() string {
	unit, ok := supportedUnits[s.Unit]
	if !ok {
		// "unitless" is a valid histogram unit type. Returning "unitless" is
		// preferable to throwing an error here.
		return "unitless"
	}
	switch s.Direction {
	case BiggerIsBetter:
		unit += "_biggerIsBetter"
	case SmallerIsBetter:
		unit += "_smallerIsBetter"
	}
	return unit
}

// Values holds performance metric values.
type Values struct {
	values map[Metric][]float64
}

// NewValues returns a new empty Values.
func NewValues() *Values {
	return &Values{values: make(map[Metric][]float64)}
}

// Merge merges all data points of vs into this Values structure.
func (p *Values) Merge(vs ...*Values) {
	for _, val := range vs {
		for k, v := range val.values {
			if k.Multiple {
				p.Append(k, v...)
			} else {
				if _, c := p.values[k]; c {
					panic("Single-valued metric already present. Cannot merge with another value.")
				}
				p.Set(k, v...)
			}
		}
	}
}

// NewValuesFromProto creates a Values from a perfpf.Values.
func NewValuesFromProto(vs ...*perfpb.Values) *Values {
	p := NewValues()
	for _, val := range vs {
		for _, v := range val.Values {
			m := Metric{
				Name:      v.Name,
				Variant:   v.Variant,
				Unit:      v.Unit,
				Direction: Direction(v.Direction),
				Multiple:  v.Multiple,
			}
			if v.Multiple {
				p.Append(m, v.Value...)
			} else {
				p.Set(m, v.Value...)
			}
		}
	}
	return p
}

// Append appends performance metrics values. It can be called only for multi-valued
// performance metrics.
func (p *Values) Append(s Metric, vs ...float64) {
	s.setDefaults()
	if !s.Multiple {
		panic("Append must not be called for single-valued data series")
	}
	p.values[s] = append(p.values[s], vs...)
	validate(s, p.values[s])
}

// Set sets a performance metric value(s).
func (p *Values) Set(s Metric, vs ...float64) {
	s.setDefaults()
	p.values[s] = vs
	validate(s, p.values[s])
}

// Format describes the output format for perf data.
type Format int

const (
	// Crosbolt is used for Chrome OS infra dashboards (go/crosbolt).
	Crosbolt Format = iota
	// Chromeperf is used for Chrome OS infra dashboards (go/chromeperf).
	Chromeperf
)

func (format Format) fileName() (string, error) {
	switch format {
	case Crosbolt:
		return "results-chart.json", nil
	case Chromeperf:
		return "perf_results.json", nil
	default:
		return "", errors.Errorf("invalid perf format: %d", format)
	}
}

// traceData is a struct corresponding to a trace entry in Chrome Performance Dashboard JSON.
// See: https://github.com/catapult-project/catapult/blob/master/dashboard/docs/data-format.md
type traceData struct {
	Units                string `json:"units"`
	ImprovementDirection string `json:"improvement_direction"`
	Type                 string `json:"type"`

	// These are pointers to permit us to include zero values in JSON representations.
	Value  *float64   `json:"value,omitempty"`
	Values *[]float64 `json:"values,omitempty"`
}

// diagnostic corresponds to the catapult Diagnostic struct preferred by
// go/chromeperf. For more info see:
// https://chromium.googlesource.com/catapult/+/HEAD/docs/histogram-set-json-format.md
// https://chromeperf.appspot.com/
type diagnostic struct {
	Type   string   `json:"type"`
	GUID   string   `json:"guid"`
	Values []string `json:"values"`
}

// diagnosticMap corresponds to the catapult DiagnosticMap struct.
type diagnosticMap struct {
	Benchmarks string `json:"benchmarks"`
}

// histogram corresponds to the catapult Histogram format preferred by
// go/chromeperf. See diagnostic struct for more info.
type histogram struct {
	Name         string        `json:"name"`
	Unit         string        `json:"unit"`
	Diagnostics  diagnosticMap `json:"diagnostics"`
	SampleValues []float64     `json:"sampleValues"`
	Running      [7]float64    `json:"running"`
	AllBins      [][]int       `json:"allBins"`
}

func (h *histogram) updateRunning() {
	sorted := h.SampleValues
	sort.Float64s(sorted)
	min := sorted[0]
	max := sorted[len(sorted)-1]
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(len(sorted))
	variance := 0.0
	for _, v := range sorted {
		d := v - mean
		variance += d * d
	}
	h.Running = [7]float64{float64(len(sorted)), max, math.Log(mean), mean, min, sum, variance}
}

// toCrosbolt returns perf values formatted as json for crosbolt.
func (p *Values) toCrosbolt() ([]byte, error) {
	charts := &map[string]*map[string]*traceData{}

	for s := range p.values {
		// Need the original slice since we'll take a pointer to it.
		vs := p.values[s]

		// Avoid nil slices since they are encoded to null.
		if vs == nil {
			vs = []float64{}
		}

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
			t.Values = &vs
		} else {
			t.Type = "scalar"
			t.Value = &vs[0]
		}

		(*traces)[s.Variant] = &t
	}

	return json.MarshalIndent(charts, "", "  ")
}

// toChromeperf returns perf values formatted as json for chromeperf.
func (p *Values) toChromeperf(ctx context.Context) ([]byte, error) {
	guid, err := runGenGUID(ctx)
	if err != nil {
		return nil, err
	}

	diag := diagnostic{
		Type:   "GenericSet",
		GUID:   guid,
		Values: []string{"disk_image_size"},
	}

	hgrams := map[string]histogram{}
	for s, vs := range p.values {
		if vs == nil {
			continue
		}
		h, ok := hgrams[s.Name]
		if ok {
			// TODO(stevenjb): Handle Variances and resolve mismatched units.
			h.SampleValues = append(h.SampleValues, vs...)
			h.updateRunning()
		} else {
			h = histogram{
				Name:         s.Name,
				Unit:         s.histogramUnit(),
				Diagnostics:  diagnosticMap{Benchmarks: diag.GUID},
				SampleValues: vs,
				AllBins:      [][]int{{1}},
			}
			h.updateRunning()
			hgrams[s.Name] = h
		}
	}

	// The json file format is an array of diagnostic and histogram structs.
	var data []interface{}

	// Make diag the first entry.
	data = append(data, diag)

	// Append the hgrams entries in deterministic (Name) order.
	var keys []string
	for k := range hgrams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		data = append(data, hgrams[k])
	}

	return json.MarshalIndent(data, "", "  ")
}

// Save saves performance metric values as a JSON file named and formatted for
// crosbolt. outDir should be the output directory path obtained from
// testing.State.
func (p *Values) Save(outDir string) error {
	fileName, err := Crosbolt.fileName()
	if err != nil {
		return err
	}
	json, err := p.toCrosbolt()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(outDir, fileName), json, 0644)
}

// Proto converts this Values to something that can be passed in a gRPC call.
func (p *Values) Proto() *perfpb.Values {
	result := &perfpb.Values{}
	for k, v := range p.values {
		result.Values = append(result.Values, &perfpb.Value{
			Name:      k.Name,
			Variant:   k.Variant,
			Unit:      k.Unit,
			Direction: perfpb.Direction(k.Direction),
			Multiple:  k.Multiple,
			Value:     v,
		})
	}
	return result
}

// SaveAs saves performance metric values in the format provided to outDir.
// outDir should be the output directory path obtained from testing.State.
// format must be either Crosbolt or Chromeperf.
func (p *Values) SaveAs(ctx context.Context, outDir string, format Format) error {
	fileName, err := format.fileName()
	if err != nil {
		return err
	}

	var json []byte
	switch format {
	case Crosbolt:
		json, err = p.toCrosbolt()
	case Chromeperf:
		json, err = p.toChromeperf(ctx)
	}

	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(outDir, fileName), json, 0644)
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
