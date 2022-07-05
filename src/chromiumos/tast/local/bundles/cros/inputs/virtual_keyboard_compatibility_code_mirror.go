// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardCompatibilityCodeMirror,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that keyboard input works in the CodeMirror code editor",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Data:         []string{"virtual_keyboard_compatibility_code_mirror.html", "third_party/codemirror/codemirror.min.js", "third_party/codemirror/codemirror.min.css"},
		SearchFlags: []*testing.StringPair{
			{
				Key:   "ime",
				Value: ime.EnglishUS.Name,
			},
		},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.TabletVK,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func VirtualKeyboardCompatibilityCodeMirror(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowserWithServer(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn,
		httptest.NewServer(http.FileServer(s.DataFileSystem())), "virtual_keyboard_compatibility_code_mirror.html")
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	ui := uiauto.New(tconn)
	vkbCtx := vkb.NewContext(cr, tconn)

	// In this test, the CodeMirror editor is initialized with "axx');" in the text field.
	// See data/app_compatibility_code_mirror.html for the setup code.
	existingTextFinder := nodewith.Name("axx');").Role(role.StaticText)
	expectedTextFinder := nodewith.Name("a.b('x');").Role(role.StaticText)
	if err := uiauto.UserAction(
		"App Compatibility - CodeMirror",
		uiauto.Combine("click the CodeMirror editor, edit the text inside it, and validate the result",
			// Because CodeMirror uses a monospace font, we can expect the cursor to be in the exact middle of the existing text.
			vkbCtx.ClickUntilVKShown(existingTextFinder),
			vkbCtx.TapKeys([]string{"space", "backspace", "backspace", "backspace", ".", "b", "switch to symbols", "(", "'", "switch to letters", "enter", "backspace", "x"}),
			vkbCtx.HideVirtualKeyboard(),
			ui.WaitUntilExists(expectedTextFinder),
		),
		uc, &useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: "VK typing on CodeMirror",
				useractions.AttributeInputField:   "codeMirror",
				useractions.AttributeFeature:      useractions.FeatureVKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate CodeMirror typing: ", err)
	}
}
