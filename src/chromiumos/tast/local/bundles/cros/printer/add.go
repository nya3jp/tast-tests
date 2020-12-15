// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ippprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// base adds two parameterized tests, one prefixed with "proxy_" that uses
// the CUPS proxy for printing and the other that does not.
func base(extraAttr []string, printFile, name, ppdFile, goldenFile string, options []ippprint.Option) []testing.Param {
	return []testing.Param{{
		Name: name,
		Val: &ippprint.Params{
			PpdFile:      ppdFile,
			PrintFile:    printFile,
			ExpectedFile: goldenFile,
			Options:      options,
		},
		ExtraData: []string{printFile, ppdFile, goldenFile},
		ExtraAttr: extraAttr,
	}, {
		Name: "proxy_" + name,
		Val: &ippprint.Params{
			PpdFile:      ppdFile,
			PrintFile:    printFile,
			ExpectedFile: goldenFile,
			Options:      options,
			UseProxy:     true,
		},
		ExtraData:         []string{printFile, ppdFile, goldenFile},
		ExtraAttr:         extraAttr,
		ExtraSoftwareDeps: []string{"chrome", "plugin_vm"},
		Pre:               chrome.LoggedIn(),
	}}
}

// test adds non-informational parametrized tests (one proxy, one regular)
// that use "to_print.pdf" for printing.
func test(name, ppdFile, goldenFile string, options ...ippprint.Option) []testing.Param {
	return base(nil, "to_print.pdf", name, ppdFile, goldenFile, options)
}

// itest adds informational parametrized tests (one proxy, one regular)
// that use "to_print.pdf" for printing.
func itest(name, ppdFile, goldenFile string, options ...ippprint.Option) []testing.Param {
	return base([]string{"informational"}, "to_print.pdf", name, ppdFile, goldenFile, options)
}

// test2 adds non-informational parametrized tests (one proxy, one regular)
// that use "2page.pdf" for printing.
func test2(name, ppdFile, goldenFile string, options ...ippprint.Option) []testing.Param {
	return base(nil, "2page.pdf", name, ppdFile, goldenFile, options)
}

// itest2 adds informational parametrized tests (one proxy, one regular)
// that use "2page.pdf" for printing.
func itest2(name, ppdFile, goldenFile string, options ...ippprint.Option) []testing.Param {
	return base([]string{"informational"}, "2page.pdf", name, ppdFile, goldenFile, options)
}

func flatten(params [][]testing.Param) []testing.Param {
	var r []testing.Param
	for _, p := range params {
		r = append(r, p...)
	}
	return r
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Add,
		Desc: "Verifies the lp command enqueues print jobs",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Params: flatten([][]testing.Param{
			// Collate
			itest2("epson_software_collate", "printer_EpsonWF3620.ppd", "printer_collate_epson_software_collate_golden.bin", ippprint.WithCopies(2), ippprint.Collate()),
			itest2("epson_software_uncollated", "printer_EpsonWF3620.ppd", "printer_collate_epson_software_uncollated_golden.bin", ippprint.WithCopies(2)),
			itest2("epson_hardware_collate", "printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_collate_golden.ps", ippprint.WithCopies(2), ippprint.Collate()),
			itest2("epson_hardware_uncollated", "printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_uncollated_golden.ps", ippprint.WithCopies(2)),

			// Resolution
			itest("lexmark_600dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_600dpi_golden.ps", ippprint.WithResolution("600dpi")),
			itest("lexmark_1200dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_1200dpi_golden.ps", ippprint.WithResolution("1200dpi")),
			itest("lexmark_2400x600dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_2400x600dpi_golden.ps", ippprint.WithResolution("2400x600dpi")),

			// Add
			itest("dymo_lw", "printer_add_dymo_printer_lw450.ppd", "printer_add_dymo_lw_printer_golden.bin"),
			itest("dymo_lm", "printer_add_dymo_printer_lm450.ppd", "printer_add_dymo_lm_printer_golden.bin"),
			itest("epson", "printer_EpsonWF3620.ppd", "printer_add_epson_printer_golden.ps"),
			itest("epson_color", "printer_EpsonGenericColorModel.ppd", "printer_add_epson_printer_color_golden.bin", "print-color-mode=color"),
			itest("epson_monochrome", "printer_EpsonGenericColorModel.ppd", "printer_add_epson_printer_monochrome_golden.bin", "print-color-mode=monochrome"),
			test("generic", "printer_add_generic_printer_GenericPostScript.ppd.gz", "printer_add_generic_printer_golden.ps"),
			itest("hp", "printer_add_hp_printer_pclm.ppd.gz", "printer_add_hp_printer_pclm_out.pclm"),
			itest("hp_pwg_raster_color", "hp_ipp_everywhere.ppd", "printer_add_hp_ipp_everywhere_golden.pwg"),
			itest("hp_pwg_raster_monochrome", "hp_ipp_everywhere.ppd", "printer_add_hp_pwg_raster_monochrome_golden.pwg", "print-color-mode=monochrome"),
			test("star", "printer_add_star_printer_rastertostar.ppd.gz", "printer_add_star_printer_rastertostar.bin"),
			test("star_lm", "printer_add_star_printer_rastertostarlm.ppd.gz", "printer_add_star_printer_rastertostarlm.bin"),

			// Pin print
			test("hp_no_pin", "printer_HP.ppd", "printer_pin_print_hp_no_pin_golden.ps"),
			test("hp_pin", "printer_HP.ppd", "printer_pin_print_hp_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("lexmark_no_pin", "printer_Lexmark.ppd", "printer_pin_print_lexmark_no_pin_golden.ps"),
			test("lexmark_pin", "printer_Lexmark.ppd", "printer_pin_print_lexmark_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("ricoh_jobpassword_no_pin", "printer_Ricoh_JobPassword.ppd", "printer_pin_print_ricoh_JobPassword_no_pin_golden.ps"),
			test("ricoh_jobpassword_pin", "printer_Ricoh_JobPassword.ppd", "printer_pin_print_ricoh_JobPassword_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("ricoh_lockedprintpassword_nopin", "printer_Ricoh_LockedPrintPassword.ppd", "printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps"),
			test("ricoh_lockedprintpassword_pin", "printer_Ricoh_LockedPrintPassword.ppd", "printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("ricoh_password_no_pin", "printer_Ricoh_password.ppd", "printer_pin_print_ricoh_password_no_pin_golden.ps"),
			test("ricoh_password_pin", "printer_Ricoh_password.ppd", "printer_pin_print_ricoh_password_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("sharp_no_pin", "printer_Sharp.ppd", "printer_pin_print_sharp_no_pin_golden.ps"),
			test("sharp_pin", "printer_Sharp.ppd", "printer_pin_print_sharp_pin_golden.ps", ippprint.WithJobPassword("1234")),
			test("unsupported_no_pin", "printer_unsupported_GenericPostScript.ppd.gz", "printer_pin_print_unsupported_golden.ps"),
			test("unsupported_pin", "printer_unsupported_GenericPostScript.ppd.gz", "printer_pin_print_unsupported_golden.ps", ippprint.WithJobPassword("1234")),
		})})
}

func Add(ctx context.Context, s *testing.State) {
	ippprint.Run(ctx, s, s.Param().(*ippprint.Params))
}
