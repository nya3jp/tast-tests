// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/vm"
)

// Param is like testing.Param, but without fields that cannot be
// supported by MakeTestParamsFromList, such as preconditions and
// hardware dependencies
type Param struct {
	// Name of the test case. Generated tests will look like "name_artifact", "name_download_buster" etc.
	Name string

	// ExtraAttr contains additional attributes to add to the
	// generated test's ExtraAttr field beyond what the generator
	// function adds. If you want your tests to be off of the CQ,
	// add "informational" here.
	ExtraAttr []string

	// ExtraData contains paths of additional data files needed by
	// the test case. Note that data files required for specific
	// crostini preconditions are added automatically to the
	// generated tests and should not be added here.
	ExtraData []string

	// ExtraSoftwareDeps lists software features that are required
	// to run this test case.
	ExtraSoftwareDeps []string

	// Timeout indicates the timeout for this test case. This is
	// used directly for artifact tests, download tests add 3
	// minutes to allow additional time for the VM and container to
	// be downloaded. If unspecified, defaults to 7 * time.Minute.
	Timeout time.Duration

	// Val is a freeform value that can be retrieved from
	// testing.State.Param() method. This string is inserted
	// unmodified and unquoted into the generated test cases.
	Val string

	// MinimalSet - if set, generate only a minimal set of test
	// parameters such that each device will have at most one test
	// case it can run. This is useful for things like performance
	// tests, which are too expensive to be run in every possible
	// configuration.
	MinimalSet bool
}

type generatedParam struct {
	Name              string
	ExtraAttr         []string
	ExtraData         []string
	ExtraSoftwareDeps []string
	ExtraHardwareDeps string
	Pre               string
	Timeout           time.Duration
	Val               string
}

const template = `
{{range .}} {
	{{if .Name}}              Name:              {{fmt .Name}},                                             {{end}}
	{{if .ExtraAttr}}         ExtraAttr:         []string{ {{range .ExtraAttr}}{{fmt .}},{{end}} },         {{end}}
	{{if .ExtraData}}         ExtraData:         []string{ {{range .ExtraData}}{{fmt .}},{{end}} },         {{end}}
	{{if .ExtraSoftwareDeps}} ExtraSoftwareDeps: []string{ {{range .ExtraSoftwareDeps}}{{fmt .}},{{end}} }, {{end}}
	{{if .ExtraHardwareDeps}} ExtraHardwareDeps: {{.ExtraHardwareDeps}},                                    {{end}}
	{{if .Pre}}               Pre:               {{.Pre}},                                                  {{end}}
	{{if .Timeout}}           Timeout:           {{fmt .Timeout}},                                          {{end}}
	{{if .Val}}               Val:               {{.Val}},                                                  {{end}}
}, {{end}}
`

// MakeTestParamsFromList takes a list of test cases (in the form of
// crostini.Param objects) and generates a full set of crostini tests
// for each. Currently this means all four of artifact,
// artifact_unstable, download_stretch, and download_buster tests. If
// the input items have values assigned to their parameters, these
// will be merged into the output test cases.
//
// In particular, if a particular test case should not be cq-critical
// (e.g. all new tests), add "informational" to ExtraAttr for that
// case. Otherwise the generated tests which are eligible for being on
// the CQ (not unstable or download tests) will be made cq-critical
// (unless the test is tagged as informational outside of the params
// list).
//
// Normally you should use MakeTestParams instead, but if your test is
// parameterized beyond which crostini preconditions it uses, you will
// need this.
func MakeTestParamsFromList(t genparams.TestingT, baseCases []Param) string {
	var result []generatedParam
	for _, testCase := range baseCases {
		var namePrefix string
		if testCase.Name != "" {
			namePrefix = testCase.Name + "_"
		}

		// Check here if it's possible for any iteration of
		// this test to be critical, i.e. if it doesn't
		// already have the "informational" attribute.
		canBeCritical := true
		for _, attr := range testCase.ExtraAttr {
			if attr == "informational" {
				canBeCritical = false
			}
		}

		for _, stable := range []bool{true, false} {
			name := "artifact"
			if !stable {
				name += "_unstable"
			}

			// _unstable tests can never be CQ critical.
			var extraAttr []string
			if !stable && canBeCritical {
				extraAttr = append(extraAttr, "informational")
			}

			extraData := []string{ImageArtifact}

			var hardwareDeps string
			if stable {
				hardwareDeps = "crostini.CrostiniStable"
			} else {
				hardwareDeps = "crostini.CrostiniUnstable"
			}

			var timeout time.Duration
			if testCase.Timeout != time.Duration(0) {
				timeout = testCase.Timeout
			} else {
				timeout = 7 * time.Minute
			}

			testParam := generatedParam{
				Name:              namePrefix + name,
				ExtraAttr:         append(testCase.ExtraAttr, extraAttr...),
				ExtraData:         append(testCase.ExtraData, extraData...),
				ExtraSoftwareDeps: testCase.ExtraSoftwareDeps,
				ExtraHardwareDeps: hardwareDeps,
				Pre:               "crostini.StartedByArtifact()",
				Timeout:           timeout,
				Val:               testCase.Val,
			}
			result = append(result, testParam)
		}

		if testCase.MinimalSet {
			continue
		}

		for _, debianVersion := range []vm.ContainerDebianVersion{vm.DebianStretch, vm.DebianBuster} {
			name := "download_" + string(debianVersion)

			var extraAttr []string
			// Download tests can never be CQ critical.
			if canBeCritical {
				extraAttr = append(extraAttr, "informational")
			}

			var pre string
			if debianVersion == vm.DebianStretch {
				pre = "crostini.StartedByDownloadStretch()"
			} else {
				pre = "crostini.StartedByDownloadBuster()"
			}

			var timeout time.Duration
			if testCase.Timeout != time.Duration(0) {
				timeout = testCase.Timeout + 3*time.Minute
			} else {
				timeout = 10 * time.Minute
			}

			testParam := generatedParam{
				Name:              namePrefix + name,
				ExtraAttr:         append(testCase.ExtraAttr, extraAttr...),
				ExtraData:         testCase.ExtraData,
				ExtraSoftwareDeps: testCase.ExtraSoftwareDeps,
				Pre:               pre,
				Timeout:           timeout,
				Val:               testCase.Val,
			}
			result = append(result, testParam)
		}

	}
	return genparams.Template(t, template, result)
}

// MakeTestParams generates the default set of crostini test
// parameters using MakeTestParamsFromList. If your test only needs to
// be parameterized over how crostini is acquired and which version is
// installed, use this. Otherwise, you may need to use
// MakeTestParamsFromList.
//
// Sub-tests which are not eligible for being on the CQ (unstable or
// download tests) will be tagged informational. Whether the test as a
// whole is CQ-critical should be controlled by a test-level
// informational attribute.
func MakeTestParams(t genparams.TestingT) string {
	defaultTest := Param{}
	return MakeTestParamsFromList(t, []Param{defaultTest})
}
