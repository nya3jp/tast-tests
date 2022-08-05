// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintCombinations,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that settings can be set in the print dialog",
		Contacts:     []string{"project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"chrome", "cros_internal", "cups"},
		Params: []testing.Param{
			// Format for test cases is as follows:
			// Name: manufacturer_model
			// Val: printer descriptor file
			// ExtraData: printer descriptor file
			{
				//MFP in test lab
				Name:      "sharp_mx_b467f",
				Val:       "sharp_mx_b467f_descriptor.json",
				ExtraData: []string{"sharp_mx_b467f_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in test lab
				Name:      "hp_laserjet_pro_m478",
				Val:       "hp_laserjet_pro_m478_descriptor.json",
				ExtraData: []string{"hp_laserjet_pro_m478_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in test lab
				Name:      "brother_dcp_l2550dw",
				Val:       "brother_dcp_l2550dw_descriptor.json",
				ExtraData: []string{"brother_dcp_l2550dw_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			},
			// printers in BLD lab: un-comment to test them
			/*{
				//MFP in BLD lab
				Name:      "lexmark_mb2236adwe",
				Val:       "lexmark_mb2236adwe_descriptor.json",
				ExtraData: []string{"lexmark_mb2236adwe_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in BLD lab
				Name:      "epson_xp_7100_series",
				Val:       "epson_xp_7100_series_descriptor.json",
				ExtraData: []string{"epson_xp_7100_series_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in BLD lab
				Name:      "epson_wf_2540_series",
				Val:       "epson_wf_2540_series_descriptor.json",
				ExtraData: []string{"epson_wf_2540_series_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in BLD lab
				Name:      "epson_artisan_837",
				Val:       "epson_artisan_837_descriptor.json",
				ExtraData: []string{"epson_artisan_837_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			}, {
				//MFP in BLD lab
				Name:      "brother_hl_l2395dw_series",
				Val:       "brother_hl_l2395dw_series_descriptor.json",
				ExtraData: []string{"brother_hl_l2395dw_series_descriptor.json"},
				ExtraAttr: []string{"paper-io_mfp_printscan"},
			},*/
		},
	})
}

func PrintCombinations(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Open a new Chrome tab so that there's something to print.
	ui := uiauto.New(tconn)
	if _, err := cr.NewConn(ctx, "chrome://newtab"); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	// Read and parse the JSON file describing the printer's available settings.
	fileContents, err := ioutil.ReadFile(s.DataPath(s.Param().(string)))
	if err != nil {
		s.Fatal("Unable to read printer descriptor file: ", err)
	}
	var desc printerDescription
	if err := json.Unmarshal(fileContents, &desc); err != nil {
		s.Fatal("Failed to parse printer's JSON description: ", err)
	}
	printerName := desc.Name

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Combine("open Print Preview with shortcut Ctrl+P",
		kb.AccelAction("Ctrl+P"),
	)(ctx); err != nil {
		s.Fatal("Failed to open Print Preview: ", err)
	}

	// Select printer.
	s.Log("Selecting printer: ", printerName)
	if err := printpreview.SelectPrinter(ctx, tconn, printerName); err != nil {
		s.Fatal("Failed to select printer: ", err)
	}

	if err := printpreview.ExpandMoreSettings(ctx, tconn); err != nil {
		s.Fatal("Failed to expand 'more settings': ", err)
	}

	printButton := nodewith.Name("Print").Role(role.Button)

	// Test each value for each dropdown setting.
	for name, info := range desc.RegularSettings.Dropdowns {
		if err := setDependencies(ctx, tconn, info.Dependencies); err != nil {
			s.Fatal("Failed to set dependencies: ", err)
		}
		for _, value := range info.Values {
			s.Log("Setting dropdown '", name, "' to '", value, "'")
			if err := printpreview.SetDropdown(ctx, tconn, name, value); err != nil {
				s.Fatal("Failed to set value of dropdown '", name, "' to '", value, "': ", err)
			}
			if err := uiauto.Combine("ensure Print button is focusable",
				ui.EnsureFocused(printButton),
			)(ctx); err != nil {
				s.Fatal("Failed to focus on print button: ", err)
			}
		}
	}

	// Test both values (on and off) for each checkbox setting.
	for name, info := range desc.RegularSettings.Checkboxes {
		if err := setDependencies(ctx, tconn, info.Dependencies); err != nil {
			s.Fatal("Failed to set dependencies: ", err)
		}
		for _, value := range []bool{true, false} {
			s.Log("Setting checkbox '", name, "' to '", value, "'")
			if err := printpreview.SetCheckboxState(ctx, tconn, name, value); err != nil {
				s.Fatal("Failed to set value of checkbox '", name, "' to '", value, "': ", err)
			}
			if err := uiauto.Combine("ensure Print button is focusable",
				ui.EnsureFocused(printButton),
			)(ctx); err != nil {
				s.Fatal("Failed to focus on print button: ", err)
			}
		}
	}

	// Test each value for each advanced setting.
	for name, values := range desc.AdvancedSettings {
		for _, value := range values.([]interface{}) {
			s.Log("Set advanced setting '", name, "' to '", value, "'")
			if err := printpreview.OpenAdvancedSettings(ctx, tconn); err != nil {
				s.Fatal("Failed to open advanced settings dialog: ", err)
			}
			if err := printpreview.SetAdvancedSetting(ctx, tconn, name, value.(string)); err != nil {
				s.Fatal("Failed to set advanced setting: ", err)
			}
			if err := printpreview.CloseAdvancedSettings(ctx, tconn); err != nil {
				s.Fatal("Failed to close advanced settings dialog: ", err)
			}
			if err := uiauto.Combine("ensure Print button is focusable",
				ui.EnsureFocused(printButton),
			)(ctx); err != nil {
				s.Fatal("Failed to focus on print button: ", err)
			}
		}
	}
}

type printerDescription struct {
	Name             string
	RegularSettings  regularSettings
	AdvancedSettings map[string]interface{}
}

type regularSettings struct {
	Dropdowns  map[string]dropdownSetting
	Checkboxes map[string]checkboxSetting
}

type dropdownSetting struct {
	Type         string
	Values       []string
	Dependencies map[string]interface{}
}

type checkboxSetting struct {
	Dependencies map[string]interface{}
}

func setDependencies(ctx context.Context, tconn *chrome.TestConn, dependencies map[string]interface{}) error {
	for name, value := range dependencies {
		var err error
		switch value.(type) {
		case bool:
			err = printpreview.SetCheckboxState(ctx, tconn, name, value.(bool))
		case string:
			err = printpreview.SetDropdown(ctx, tconn, name, value.(string))
		default:
			err = errors.New("unknown dependency type")
		}
		if err != nil {
			return err
		}
	}

	return nil
}
