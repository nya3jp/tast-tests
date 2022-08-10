// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprisecuj

import (
	"context"

	"chromiumos/tast/local/apps"
	cx "chromiumos/tast/local/bundles/cros/spera/enterprisecuj/citrix"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// ClinicianWorkstationData lists all the data used in the clinician workstation cuj.
var ClinicianWorkstationData = []string{
	// The following are icons used by uidetection in the clinician workstation cuj.
	cx.IconToolbar,
	cx.IconSearch,
	cx.IconDesktop,
	cx.IconEndTask,
	cx.IconTrackerCancel,
	cx.IconChromeTaskManager,
	cx.IconChromeActive,
	cx.IconChromeWikiSearch,
	cx.IconChromeGoogleSearch,
	cx.IconPhotosUpload,
	cx.IconPhotosUploadSmall,
	cx.IconPhotosComputer,
	cx.IconPhotosDownload,
	cx.IconPhotosDelete,
}

// Test scenario for clinician workstation CUJ:
// 1. Maximize the Citrix app.
// 2. Open 3 browser windows x 2 tabs inside Citrix. Substitute Google, Google Photos, Wikipedia, WebMD for health provider website.
// 3. Do a search in wikipedia and another in google (substitue for health query).
// 4. Switch between browser windows/tabs.
// 5. Concurrently Open google keep and type notes at 70 wpm.
// 6. Upload to google photos.
// 7. Logout and login again.

// Run runs scenario for clinician workstation cuj.
func (c *ClinicianWorkstationScenario) Run(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, citrix *cx.Citrix, p *TestParams) error {
	const (
		searchTerm  = "health"
		noteContent = "Sample note"
		filename    = "sample"
	)
	// Substitute Google, Google Photos, Wikipedia, WebMD for health provider website.
	firstChromeURLs := []string{cuj.GoogleURL, cuj.GooglePhotosURL}
	secondChromeURLs := []string{cuj.WikipediaURL, cuj.WebMDURL}
	thirdChromeURLs := []string{cuj.GoogleURL, cuj.WikipediaURL}
	windows := [][]string{firstChromeURLs, secondChromeURLs, thirdChromeURLs}
	openWindows := func(ctx context.Context) error {
		for _, chromeURLs := range windows {
			if err := citrix.OpenChromeWithURLs(chromeURLs)(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	switchWindowAndAllTabs := func(ctx context.Context) error {
		for _, chromeURLs := range windows {
			if err := citrix.SwitchWindow()(ctx); err != nil {
				return err
			}
			for i := 0; i < len(chromeURLs)-1; i++ {
				if err := citrix.SwitchTab()(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return uiauto.NamedCombine("run the clinician workstation cuj scenario",
		citrix.ConnectRemoteDesktop(p.DesktopName),
		citrix.CloseAllChromeBrowsers(),
		// 1. Maximize the Citrix app.
		citrix.FullscreenDesktop(),
		citrix.FocusOnDesktop(),
		// 2. Open 3 browser windows x 2 tabs inside Citrix.
		openWindows,
		// 3. Do a search in wikipedia and another in google (substitue for health query).
		citrix.SearchFromWiki(searchTerm),
		kb.AccelAction("Ctrl+1"), // Switch to Google page.
		citrix.SearchFromGoogle(searchTerm),
		// 4. Switch between browser windows/tabs.
		switchWindowAndAllTabs,
		// 5. Concurrently Open google keep and type notes at 70 wpm.
		citrix.CreateGoogleKeepNote(noteContent),
		citrix.DeleteGoogleKeepNote(noteContent),
		// 6. Upload photo to google photos.
		citrix.UploadPhoto(filename),
		citrix.DeletePhoto(),
		citrix.CloseAllChromeBrowsers(),
		// 7. Logout and login again.
		citrix.ExitFullscreenDesktop(),
		p.UIHandler.SwitchToAppWindow(apps.Citrix.Name),
		citrix.Logout(),
		citrix.Login(p.CitrixServerURL, p.CitrixUserName, p.CitrixPassword),
	)(ctx)
}

// ClinicianWorkstationScenario implements the CitrixScenario interface.
type ClinicianWorkstationScenario struct{}

var _ CitrixScenario = (*ClinicianWorkstationScenario)(nil)

// NewClinicianWorkstationScenario creates Clinician Workstation instance which implements CitrixScenario interface.
func NewClinicianWorkstationScenario() *ClinicianWorkstationScenario {
	return &ClinicianWorkstationScenario{}
}
