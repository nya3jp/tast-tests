// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	"chromiumos/tast/testing/hwdep"
)

// This list was fetched by Plx script, use "Asus" as primaryOemName:
// https://plx.corp.google.com/scripts2/script_61._62eca9_0000_2e5b_930b_30fd38187cc4
var asusModelList = []string{
	"ampton",
	"amptone",
	"apel",
	"apel-e",
	"babymako",
	"babymega",
	"babytiger",
	"basking",
	"bob",
	"cave",
	"cerise",
	"collis",
	"copano",
	"corori",
	"corori360",
	"damu",
	"delbin",
	"delbin",
	"delbing",
	"drobit",
	"duffy",
	"dumo",
	"faffy",
	"felwinter",
	"galith",
	"galith360",
	"gallop",
	"galnat",
	"galnat360",
	"galtic",
	"galtic360",
	"hayato",
	"helios",
	"homsar",
	"jelboz",
	"jelboz360",
	"kakadu",
	"katsu",
	"kitefin",
	"leona",
	"longfin",
	"mickey",
	"nospike",
	"rabbid",
	"rabbid-ruggedized",
	"sailfin",
	"shuboz",
	"shyvana",
	"shyvana-m",
	"stern",
	"storo",
	"storo360",
	"teemo",
	"telesu",
	"terra",
	"terra13",
	"woomax",
}

// AsusModels returns hardwareDeps condition with list of all Asus models.
func AsusModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(asusModelList...))
}
