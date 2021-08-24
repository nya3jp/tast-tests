// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Run "TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test add_test.go"
// to regenerate parameters for add.go, proxy_add.go.

package printer

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/bundles/cros/printer/ippprint"
)

// base adds two parameterized tests, one that uses the CUPS proxy for
// printing and the other that does not.
type base struct {
	PrintFile, Name, PPDFile, ExpectedFile string
	ExtraAttr, Options                     []string
}

// test adds non-informational parametrized tests (one proxy, one regular)
// that use "to_print.pdf" for printing.
func test(name, ppdFile, expectedFile string, options ...string) base {
	return base{PrintFile: "to_print.pdf", Name: name, PPDFile: ppdFile, ExpectedFile: expectedFile, Options: options}
}

// iTest adds informational parametrized tests (one proxy, one regular)
// that use "to_print.pdf" for printing.
func iTest(name, ppdFile, expectedFile string, options ...string) base {
	return base{ExtraAttr: []string{"informational"}, PrintFile: "to_print.pdf", Name: name, PPDFile: ppdFile, ExpectedFile: expectedFile, Options: options}
}

// test2 adds non-informational parametrized tests (one proxy, one regular)
// that use "2page.pdf" for printing.
func test2(name, ppdFile, expectedFile string, options ...string) base {
	return base{PrintFile: "2page.pdf", Name: name, PPDFile: ppdFile, ExpectedFile: expectedFile, Options: options}
}

// iTest2 adds informational parametrized tests (one proxy, one regular)
// that use "2page.pdf" for printing.
func iTest2(name, ppdFile, expectedFile string, options ...string) base {
	return base{ExtraAttr: []string{"informational"}, PrintFile: "2page.pdf", Name: name, PPDFile: ppdFile, ExpectedFile: expectedFile, Options: options}
}

func TestAddParams(t *testing.T) {
	code := genparams.Template(t, `{{ range . }} {
        Name: {{ .Name | fmt }},
        Val: &ippprint.Params{
                PPDFile:      {{ .PPDFile | fmt }},
                PrintFile:    {{ .PrintFile | fmt }},
                ExpectedFile: {{ .ExpectedFile | fmt }},
                {{ if .Options }}
                Options:      {{ .Options | fmt }},
                {{ end }}
        },
        ExtraData: []string{ {{ .PrintFile | fmt }}, {{ .PPDFile | fmt }}, {{ .ExpectedFile | fmt }} },
        {{ if .ExtraAttr }}
        ExtraAttr: {{ .ExtraAttr | fmt }},
        {{ end }}
}, {{ end }}`, []base{
		// Collate
		test2("epson_software_collate", "printer_EpsonWF3620.ppd", "printer_collate_epson_software_collate_golden.bin", ippprint.WithCopies(2), ippprint.Collate()),
		test2("epson_software_uncollated", "printer_EpsonWF3620.ppd", "printer_collate_epson_software_uncollated_golden.bin", ippprint.WithCopies(2)),
		test2("epson_hardware_collate", "printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_collate_golden.ps", ippprint.WithCopies(2), ippprint.Collate()),
		test2("epson_hardware_uncollated", "printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_uncollated_golden.ps", ippprint.WithCopies(2)),

		// Resolution
		test("lexmark_600dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_600dpi_golden.ps", ippprint.WithResolution("600dpi")),
		test("lexmark_1200dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_1200dpi_golden.ps", ippprint.WithResolution("1200dpi")),
		test("lexmark_2400x600dpi", "printer_Lexmark.ppd", "printer_resolution_lexmark_2400x600dpi_golden.ps", ippprint.WithResolution("2400x600dpi")),

		// Verify common media-source values by-pass-tray (Multipurpose) and bottom (Lower).
		iTest("media_source_by_pass_tray", "printer_add_generic_printer_GenericPostScript.ppd.gz", "printer_add_media_source_bypass_golden.ps", "media-source=by-pass-tray"),
		iTest("media_source_bottom", "printer_add_generic_printer_GenericPostScript.ppd.gz", "printer_add_media_source_bottom_golden.ps", "media-source=bottom"),

		// Add
		test("dymo_lw", "printer_add_dymo_printer_lw450.ppd", "printer_add_dymo_lw_printer_golden.bin"),
		test("dymo_lm", "printer_add_dymo_printer_lm450.ppd", "printer_add_dymo_lm_printer_golden.bin"),
		test("epson", "printer_EpsonWF3620.ppd", "printer_add_epson_printer_golden.ps"),
		test("epson_color", "printer_EpsonGenericColorModel.ppd", "printer_add_epson_printer_color_golden.bin", "print-color-mode=color"),
		test("epson_monochrome", "printer_EpsonGenericColorModel.ppd", "printer_add_epson_printer_monochrome_golden.bin", "print-color-mode=monochrome"),
		test("generic", "printer_add_generic_printer_GenericPostScript.ppd.gz", "printer_add_generic_printer_golden.ps"),
		test("hp_pclm", "printer_add_hp_printer_pclm.ppd.gz", "printer_add_hp_printer_pclm_out.pclm"),
		iTest("hp_ljcolor", "printer_add_hp_ljcolor.ppd.gz", "printer_add_hp_printer_ljcolor_out.pcl"),
		test("hp_pwg_raster_color", "hp_ipp_everywhere.ppd", "printer_add_hp_ipp_everywhere_golden.pwg"),
		test("hp_pwg_raster_monochrome", "hp_ipp_everywhere.ppd", "printer_add_hp_pwg_raster_monochrome_golden.pwg", "print-color-mode=monochrome"),
		iTest("nec", "printer_nec_npdl.ppd", "printer_add_nec_golden.bin"),
		test("star", "printer_add_star_printer_rastertostar.ppd.gz", "printer_add_star_printer_rastertostar.bin"),
		iTest("star_m", "printer_add_star_printer_rastertostarm.ppd.gz", "printer_add_star_printer_rastertostarm.bin"),
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
	})
	genparams.Ensure(t, "add.go", code)
	genparams.Ensure(t, "proxy_add.go", code)
}
