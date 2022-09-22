// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// UIComponent represents a CCA UI component.
type UIComponent struct {
	Name      string
	Selectors []string
}

var (
	// CancelResultButton is button for canceling intent review result.
	CancelResultButton = UIComponent{"cancel result button", []string{"#cancel-result", "button[i18n-label=cancel_review_button]"}}
	// ConfirmResultButton is button for confirming intent review result.
	ConfirmResultButton = UIComponent{"confirm result button", []string{"#confirm-result", "button[i18n-label=confirm_review_button]"}}
	// MirrorButton is button used for toggling preview mirroring option.
	MirrorButton = UIComponent{"mirror button", []string{"#toggle-mirror"}}
	// ModeSelector is selection bar for different capture modes.
	ModeSelector = UIComponent{"mode selector", []string{"#modes-group"}}
	// SettingsButton is button for opening primary setting menu.
	SettingsButton = UIComponent{"settings", []string{"#open-settings"}}
	// SwitchDeviceButton is button for switching camera device.
	SwitchDeviceButton = UIComponent{"switch device button", []string{"#switch-device"}}
	// VideoSnapshotButton is button for taking video snapshot during recording.
	VideoSnapshotButton = UIComponent{"video snapshot button", []string{"#video-snapshot"}}
	// VideoPauseResumeButton is button for pausing or resuming video recording.
	VideoPauseResumeButton = UIComponent{"video pause/resume button", []string{"#pause-recordvideo"}}
	// GalleryButton is button for entering the Backlight app as a gallery for captured files.
	GalleryButton = UIComponent{"gallery button", []string{"#gallery-enter"}}
	// GalleryButtonCover is cover photo of gallery button.
	GalleryButtonCover = UIComponent{"gallery button cover", []string{"#gallery-enter>img"}}

	// PhotoResolutionSettingButton is button for opening photo resolution setting menu.
	PhotoResolutionSettingButton = UIComponent{"photo resolution setting button", []string{"#settings-photo-resolution"}}
	// PhotoAspectRatioSettingButton is button for opening photo aspect ratio setting menu.
	PhotoAspectRatioSettingButton = UIComponent{"photo aspect ratio setting button", []string{"#settings-photo-aspect-ratio"}}
	// VideoResolutionSettingButton is button for opening video resolution setting menu.
	VideoResolutionSettingButton = UIComponent{"video resolution setting button", []string{"#settings-video-resolution"}}

	// ResolutionSettingButton is button for opening resolution setting menu.
	ResolutionSettingButton = UIComponent{"resolution setting button", []string{"#settings-resolution"}}
	// ExpertModeButton is button used for opening expert mode setting menu.
	ExpertModeButton = UIComponent{"expert mode button", []string{"#settings-expert"}}
	// PhotoResolutionOption is option for each available photo capture resolution.
	PhotoResolutionOption = UIComponent{"photo resolution option", []string{
		"#view-photo-resolution-settings input"}}
	// VideoResolutionOption is option for each available video capture resolution.
	VideoResolutionOption = UIComponent{"video resolution option", []string{
		"#view-video-resolution-settings input"}}
	// FeedbackButton is the feedback button showing in the settings menu.
	FeedbackButton = UIComponent{"feedback button", []string{"#settings-feedback"}}
	// HelpButton is the help button showing in the settings menu.
	HelpButton = UIComponent{"help button", []string{"#settings-help"}}
	// GridTypeSettingsButton is the button showing in the settings menu which is used for entering the grid type settings menu.
	GridTypeSettingsButton = UIComponent{"grid type settings button", []string{"#settings-gridtype"}}
	// GoldenGridButton is the button to enable golden grid type.
	GoldenGridButton = UIComponent{"golden grid type button", []string{"#grid-golden"}}
	// TimerSettingsButton is the button showing in the settings menu which is used for entering the timer settings menu.
	TimerSettingsButton = UIComponent{"timer settings button", []string{"#settings-timerdur"}}
	// Timer10sButton is the button to enable 10s timer.
	Timer10sButton = UIComponent{"timer 10s button", []string{"#timer-10s"}}

	// BarcodeChipURL is chip for url detected from barcode.
	BarcodeChipURL = UIComponent{"barcode chip url", []string{".barcode-chip-url a"}}
	// BarcodeChipText is chip for text detected from barcode.
	BarcodeChipText = UIComponent{"barcode chip text", []string{".barcode-chip-text"}}
	// BarcodeCopyURLButton is button to copy url detected from barcode.
	BarcodeCopyURLButton = UIComponent{"barcode copy url button",
		[]string{"#barcode-chip-url-container .barcode-copy-button"}}
	// BarcodeCopyTextButton is button to copy text detected from barcode.
	BarcodeCopyTextButton = UIComponent{"barcode copy text button",
		[]string{"#barcode-chip-text-container .barcode-copy-button"}}

	// VideoProfileSelect is select-options for selecting video profile.
	VideoProfileSelect = UIComponent{"video profile select", []string{"#video-profile"}}
	// BitrateMultiplierRangeInput is range input for selecting bitrate multiplier.
	BitrateMultiplierRangeInput = UIComponent{"bitrate multiplier range input", []string{"#bitrate-slider input[type=range]"}}

	// OptionsContainer is the container for all options for opening option panel.
	OptionsContainer = UIComponent{"container of options", []string{"#options-container"}}
	// OpenMirrorPanelButton is the button which is used for opening the mirror state settings panel.
	OpenMirrorPanelButton = UIComponent{"mirror state option button", []string{"#open-mirror-panel"}}
	// OpenGridPanelButton is the button which is used for opening the grid type settings panel.
	OpenGridPanelButton = UIComponent{"grid type option button", []string{"#open-grid-panel"}}
	// OpenTimerPanelButton is the button which is used for opening the timer type settings panel.
	OpenTimerPanelButton = UIComponent{"timer type option button", []string{"#open-timer-panel"}}
	// OpenPTZPanelButton is the button for opening PTZ panel.
	OpenPTZPanelButton = UIComponent{"open ptz panel button", []string{"#open-ptz-panel"}}
	// PanLeftButton is the button for panning left preview.
	PanLeftButton = UIComponent{"pan left button", []string{"#pan-left"}}
	// PanRightButton is the button for panning right preview.
	PanRightButton = UIComponent{"pan right button", []string{"#pan-right"}}
	// TiltUpButton is the button for tilting up preview.
	TiltUpButton = UIComponent{"tilt up button", []string{"#tilt-up"}}
	// TiltDownButton is the button for tilting down preview.
	TiltDownButton = UIComponent{"tilt down button", []string{"#tilt-down"}}
	// ZoomInButton is the button for zoom in preview.
	ZoomInButton = UIComponent{"zoom in button", []string{"#zoom-in"}}
	// ZoomOutButton is the button for zoom out preview.
	ZoomOutButton = UIComponent{"zoom out button", []string{"#zoom-out"}}
	// PTZResetAllButton is the button for reset PTZ to default value.
	PTZResetAllButton = UIComponent{"ptz reset all button", []string{"#ptz-reset-all"}}

	// SquareModeButton is the button to enter square mode.
	SquareModeButton = UIComponent{"square mode button", []string{".mode-item>input[data-mode=\"square\"]"}}
	// ScanModeButton is the button to enter scan mode.
	ScanModeButton = UIComponent{"scan mode button", []string{".mode-item>input[data-mode=\"scan\"]"}}
	// ScanBarcodeOption is the option button to switch to QR code detection mode in scan mode.
	ScanBarcodeOption = UIComponent{"scan barcode option", []string{"#scan-barcode"}}
	// ScanDocumentModeOption is the document mode option of scan mode.
	ScanDocumentModeOption = UIComponent{"document mode button", []string{"#scan-document"}}
	// ReviewView is the review view after taking a photo under document mode.
	ReviewView = UIComponent{"document review view", []string{"#view-review"}}
	// ReviewImage is the image to be reviewed.
	ReviewImage = UIComponent{"review image", []string{"#view-review .review-image"}}
	// SaveAsPDFButton is the button to save document as PDF.
	SaveAsPDFButton = UIComponent{"save document as pdf button", []string{"#view-review button[i18n-text=label_save_pdf_document]"}}
	// SaveAsPhotoButton is the button to save document as photo.
	SaveAsPhotoButton = UIComponent{"save document as photo button", []string{"#view-review button[i18n-text=label_save_photo_document]"}}
	// RetakeButton is the button to retake the document photo.
	RetakeButton = UIComponent{"retake document photo button", []string{
		// TODO(b/203028477): Remove selector for old mode name after
		// naming CL on app side fully landed.
		"#review-retake", "#view-review button[i18n-text=label_retake]"}}
	// FixCropButton is the button to fix document crop area.
	FixCropButton = UIComponent{"fix document crop area button", []string{"#view-review button[i18n-text=label_fix_document]"}}
	// CropDocumentView is the view for fix document crop area.
	CropDocumentView = UIComponent{"crop document view", []string{"#view-crop-document"}}
	// CropDocumentImage is the image to be cropped document from.
	CropDocumentImage = UIComponent{"crop document image", []string{"#view-crop-document .review-image"}}
	// CropDoneButton is the button clicked after fix document crop area.
	CropDoneButton = UIComponent{"crop document done button", []string{"#view-crop-document button[i18n-text=label_crop_done]"}}
	// DocumentCorner is the dragging point of document corner in crop area page.
	DocumentCorner = UIComponent{"document corner dragging point", []string{"#view-crop-document .dot"}}
	// DocumentCornerOverlay is the overlay that CCA used to draw document corners on.
	DocumentCornerOverlay = UIComponent{"document corner overlay", []string{
		"#preview-document-corner-overlay"}}
	// DocumentDialogButton is the confirmation button of new feature dialog for document mode.
	DocumentDialogButton = UIComponent{"document feature dialog button", []string{"#view-document-mode-dialog button[i18n-text=document_mode_dialog_got_it]"}}
	// DocumentReview is the review view for multi-page document mode.
	DocumentReview = UIComponent{"document review view", []string{"#view-document-review"}}
	// DocumentPreviewModeImage is the preview image of preview mode in multi-page document mode.
	DocumentPreviewModeImage = UIComponent{"document preview mode image", []string{".document-preview-mode .image"}}
	// DocumentFixModeImage is the preview image of fix mode in multi-page document mode.
	DocumentFixModeImage = UIComponent{"document fix mode image", []string{".document-fix-mode .image"}}
	// DocumentFixButton is the entry button of fix mode in multi-page document mode.
	DocumentFixButton = UIComponent{"document enter fix mode button", []string{".document-preview-mode button[i18n-aria=label_fix_document]", ".document-preview-mode button[i18n-aria=fix_page_button]"}}
	// DocumentFixModeCorner is the crop area dragging point in fix mode in multi-page document mode.
	DocumentFixModeCorner = UIComponent{"document corner dragging point", []string{".document-fix-mode .dot"}}
	// DocumentDoneFixButton is the exit button of fix mode in multi-page document mode.
	DocumentDoneFixButton = UIComponent{"document exit fix mode button", []string{".document-fix-mode button[i18n-text=label_crop_done]"}}
	// DocumentCancelButton is the cancel button in multi-page document mode.
	DocumentCancelButton = UIComponent{"document cancel button", []string{".document-preview-mode button[i18n-text=cancel_review_button]"}}
	// DocumentBackButton is the resume button to show review UI of multi-page document mode when there're pending pages for reviewing.
	DocumentBackButton = UIComponent{"document resume button", []string{"#back-to-review-document"}}
	// DocumentAddPageButton is the button to close the review UI of multi-page document mode temporarily for adding new pages.
	DocumentAddPageButton = UIComponent{"document add page button", []string{".document-preview-mode button[i18n-aria=add_new_page_button]"}}
	// DocumentSaveAsPhotoButton is the button to save as a photo in multi-page document mode.
	DocumentSaveAsPhotoButton = UIComponent{"document save as photo button", []string{".document-preview-mode button[i18n-text=label_save_photo_document]"}}
	// DocumentSaveAsPdfButton is the button save as a PDF file in multi-page document mode.
	DocumentSaveAsPdfButton = UIComponent{"document save as PDF button", []string{".document-preview-mode button[i18n-text=label_save_pdf_document]"}}

	// GifRecordingOption is the radio button to toggle gif recording option.
	GifRecordingOption = UIComponent{"gif recording button", []string{
		"input[type=radio][data-state=record-type-gif]"}}
	// GifReviewSaveButton is the save button in gif review page.
	GifReviewSaveButton = UIComponent{"save gif button", []string{
		"#view-review button[i18n-text=label_save]"}}
	// GifReviewRetakeButton is the retake button in gif review page.
	GifReviewRetakeButton = UIComponent{"retake gif button", []string{"#review-retake"}}

	// FrontAspectRatioOptions are the buttons of aspect ratio options for the front camera.
	FrontAspectRatioOptions = UIComponent{"front aspect ratio options", []string{"#view-photo-aspect-ratio-settings .menu-item>input[data-facing=\"user\"]"}}
	// BackAspectRatioOptions are the buttons of aspect ratio options for the back camera.
	BackAspectRatioOptions = UIComponent{"back aspect ratio options", []string{"#view-photo-aspect-ratio-settings .menu-item>input[data-facing=\"environment\"]"}}
	// FrontPhotoResolutionOptions are the buttons of photo resolution options for the front camera.
	FrontPhotoResolutionOptions = UIComponent{"front photo resolution options", []string{"#view-photo-resolution-settings .menu-item>input[data-facing=\"user\"]"}}
	// BackPhotoResolutionOptions are the buttons of photo resolution options for the back camera.
	BackPhotoResolutionOptions = UIComponent{"back photo resolution options", []string{"#view-photo-resolution-settings .menu-item>input[data-facing=\"environment\"]"}}
	// FrontVideoResolutionOptions are the buttons of video resolution options for the front camera.
	FrontVideoResolutionOptions = UIComponent{"front video resolution options", []string{"#view-video-resolution-settings .menu-item>input[data-facing=\"user\"]"}}
	// BackVideoResolutionOptions are the buttons of video resolution options for the back camera.
	BackVideoResolutionOptions = UIComponent{"back video resolution options", []string{"#view-video-resolution-settings .menu-item>input[data-facing=\"environment\"]"}}
)

// Option is the option for toggling state.
type Option struct {
	// ui is the |UIComponent| to toggle the option.
	ui UIComponent
	// state is state toggle by this option.
	state string
}

func newOption(state, selector string) Option {
	name := fmt.Sprintf("option to toggle %v state", state)
	selectors := []string{selector}
	return Option{ui: UIComponent{Name: name, Selectors: selectors}, state: state}
}

var (
	// CustomVideoParametersOption is the option to enable custom video parameters.
	CustomVideoParametersOption = newOption("custom-video-parameters", "#custom-video-parameters")
	// ExpertModeOption is the option to enable expert mode.
	ExpertModeOption = newOption("expert", "#expert-enable-expert-mode")
	// GridOption is the option to show grid lines on preview.
	GridOption = newOption("grid", "#toggle-grid")
	// MirrorOption is the option to flip preview horizontally.
	MirrorOption = newOption("mirror", "#toggle-mirror")
	// SaveMetadataOption is the option to save metadata of capture result.
	SaveMetadataOption = newOption("save-metadata", "#expert-save-metadata")
	// ShowMetadataOption is the option to show preview metadata.
	ShowMetadataOption = newOption("show-metadata", "#expert-show-metadata")
	// EnableDocumentModeOnAllCamerasOption is the option to enable document scanning on all cameras.
	EnableDocumentModeOnAllCamerasOption = newOption("enable-document-mode-on-all-cameras", "#expert-enable-document-mode-on-all-cameras")
	// EnableMultistreamRecordingOption is the option to enable document scanning on all cameras.
	EnableMultistreamRecordingOption = newOption("enable-multistream-recording", "#expert-enable-multistream-recording")
	// ScanBarcodeOptionInPhotoMode is the option to enable barcode scanning in photo mode.
	ScanBarcodeOptionInPhotoMode = newOption("enable-scan-barcode", "#toggle-barcode")
	// ShowGifRecordingOption is the option to enable gif recording.
	ShowGifRecordingOption = newOption("show-gif-recording-option", "#expert-enable-gif-recording")
	// TimerOption is the option to enable countdown timer.
	TimerOption = newOption("timer", "#toggle-timer")
)

type errorUINotExist struct {
	ui *UIComponent
}

func (err errorUINotExist) Error() string {
	return fmt.Sprintf("failed to resolved ui %v to its correct selector", err.ui.Name)
}

// IsUINotExist returns true if the given error is from errorUINotExist error type.
func IsUINotExist(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(errorUINotExist); ok {
		return true
	}
	if wrappedErr, ok := err.(*errors.E); ok {
		return IsUINotExist(wrappedErr.Unwrap())
	}
	return false
}

// HasClass returns true if the given HTML element has the given class name.
func (a *App) HasClass(ctx context.Context, ui UIComponent, className string) (bool, error) {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get the selector of UI: %v", ui.Name)
	}
	var result bool
	if err := a.conn.Call(ctx, &result, "Tast.hasClass", selector, className); err != nil {
		return false, errors.Wrapf(err, "failed to check class for UI: %v and class name: %v", ui.Name, className)
	}
	return result, nil
}

// resolveUISelector resolves ui to its correct selector.
func (a *App) resolveUISelector(ctx context.Context, ui UIComponent) (string, error) {
	for _, s := range ui.Selectors {
		if exist, err := a.selectorExist(ctx, s); err != nil {
			return "", err
		} else if exist {
			return s, nil
		}
	}
	return "", errorUINotExist{ui: &ui}
}

// Style returns the value of an CSS attribute of an UI component.
func (a *App) Style(ctx context.Context, ui UIComponent, attribute string) (string, error) {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the selector of UI: %v", ui.Name)
	}
	var style string
	if err := a.conn.Call(ctx, &style, "Tast.getStyle", selector, attribute); err != nil {
		return "", errors.Wrapf(err, "failed to get the style of attribute: %v of UI: %v", attribute, ui.Name)
	}
	return style, nil
}

// Exist returns whether a UI component exists.
func (a *App) Exist(ctx context.Context, ui UIComponent) (bool, error) {
	_, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		if IsUINotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// OptionExist returns if the option exists.
func (a *App) OptionExist(ctx context.Context, option Option) (bool, error) {
	return a.Exist(ctx, option.ui)
}

// Visible returns whether a UI component is visible on the screen.
func (a *App) Visible(ctx context.Context, ui UIComponent) (bool, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to check visibility state of %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, wrapError(err)
	}
	var visible bool
	if err := a.conn.Call(ctx, &visible, "Tast.isVisible", selector); err != nil {
		return false, wrapError(err)
	}
	return visible, nil
}

// CheckVisible returns an error if visibility state of ui is not expected.
func (a *App) CheckVisible(ctx context.Context, ui UIComponent, expected bool) error {
	if visible, err := a.Visible(ctx, ui); err != nil {
		return err
	} else if visible != expected {
		return errors.Errorf("unexpected %v visibility state: got %v, want %v", ui.Name, visible, expected)
	}
	return nil
}

// WaitForVisibleState waits until the visibility of ui becomes expected.
func (a *App) WaitForVisibleState(ctx context.Context, ui UIComponent, expected bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		visible, err := a.Visible(ctx, ui)
		if err != nil {
			return testing.PollBreak(err)
		}
		if visible != expected {
			return errors.Errorf("failed to wait visibility state for %v: got %v, want %v", ui.Name, visible, expected)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// Disabled returns disabled attribute of HTMLElement of |ui|.
func (a *App) Disabled(ctx context.Context, ui UIComponent) (bool, error) {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, errors.Wrapf(err, "failed to resolve ui %v to correct selector", ui.Name)
	}
	var disabled bool
	if err := a.conn.Call(ctx, &disabled, "(selector) => document.querySelector(selector).disabled", selector); err != nil {
		return false, errors.Wrapf(err, "failed to get disabled state of %v", ui.Name)
	}
	return disabled, nil
}

// WaitForDisabled waits until the disabled state of ui becomes |expected|.
func (a *App) WaitForDisabled(ctx context.Context, ui UIComponent, expected bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		disabled, err := a.Disabled(ctx, ui)
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to wait disabled state of %v to be %v", ui.Name, expected))
		}
		if disabled != expected {
			return errors.Errorf("failed to wait disabled state for %v: got %v, want %v", ui.Name, disabled, expected)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// CountUI returns the number of ui element.
func (a *App) CountUI(ctx context.Context, ui UIComponent) (int, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to count number of %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		if IsUINotExist(err) {
			return 0, nil
		}
		return 0, wrapError(err)
	}
	var number int
	if err := a.conn.Call(ctx, &number, `(selector) => document.querySelectorAll(selector).length`, selector); err != nil {
		return 0, wrapError(err)
	}
	return number, nil
}

// AttributeWithIndex returns the attr attribute of the index th ui.
func (a *App) AttributeWithIndex(ctx context.Context, ui UIComponent, index int, attr string) (string, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to get %v attribute of %v th %v", attr, index, ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return "", wrapError(err)
	}
	var value string
	if err := a.conn.Call(
		ctx, &value,
		`(selector, index, attr) => document.querySelectorAll(selector)[index].getAttribute(attr)`,
		selector, index, attr); err != nil {
		return "", wrapError(err)
	}
	return value, nil
}

// ScreenXYWithIndex returns the screen coordinates of the left-top corner of the |index|'th |ui|.
func (a *App) ScreenXYWithIndex(ctx context.Context, ui UIComponent, index int) (*coords.Point, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to get screen coordindates of %v th %v", index, ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return nil, wrapError(err)
	}
	var pt coords.Point
	if err := a.conn.Call(ctx, &pt, `Tast.getScreenXY`, selector, index); err != nil {
		return nil, wrapError(err)
	}
	return &pt, nil
}

// Size returns size of the |ui|.
func (a *App) Size(ctx context.Context, ui UIComponent) (*Resolution, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to get size of %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return nil, wrapError(err)
	}
	var size Resolution
	if err := a.conn.Call(ctx, &size, `Tast.getSize`, selector); err != nil {
		return nil, wrapError(err)
	}
	return &size, nil
}

// Click clicks on ui.
func (a *App) Click(ctx context.Context, ui UIComponent) error {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to click on %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return wrapError(err)
	}
	if err := a.ClickWithSelector(ctx, selector); err != nil {
		return wrapError(err)
	}
	return nil
}

// ClickWithIndex clicks nth ui.
func (a *App) ClickWithIndex(ctx context.Context, ui UIComponent, index int) error {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return err
	}
	if err := a.conn.Call(ctx, nil, `(selector, index) => document.querySelectorAll(selector)[index].click()`, selector, index); err != nil {
		return errors.Wrapf(err, "failed to click on %v(th) %v", index, ui.Name)
	}
	return nil
}

// ClickChildIfContain clicks the child which contains the given string in its text content.
func (a *App) ClickChildIfContain(ctx context.Context, ui UIComponent, text string) error {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return err
	}
	if err := a.conn.Call(ctx, nil, `(selector, text) => {
		const element = document.querySelector(selector);
		const children = element.childNodes;
		const matches = [...children].filter((node) => node.textContent.includes(text));
		for (const match of matches) {
			match.click();
		}
	}`, selector, text); err != nil {
		return errors.Wrapf(err, "failed to click children of %v containing text: %v", ui.Name, text)
	}
	return nil
}

// Hold holds on |ui| by sending pointerdown and pointerup for |d| duration.
func (a *App) Hold(ctx context.Context, ui UIComponent, d time.Duration) error {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to hold on %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return wrapError(err)
	}
	return a.conn.Call(ctx, nil, `Tast.hold`, selector, d.Milliseconds())
}

// ClickPTZButton clicks on PTZ Button.
func (a *App) ClickPTZButton(ctx context.Context, ui UIComponent) error {
	// Hold for 0ms to trigger PTZ minimal step movement.
	return a.Hold(ctx, ui, 0)
}

// IsCheckedWithIndex gets checked state of nth ui.
func (a *App) IsCheckedWithIndex(ctx context.Context, ui UIComponent, index int) (bool, error) {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, err
	}
	var checked bool
	if err := a.conn.Call(ctx, &checked, `(selector, index) => document.querySelectorAll(selector)[index].checked`, selector, index); err != nil {
		return false, errors.Wrapf(err, "failed to get checked state on %v(th) %v", index, ui.Name)
	}
	return checked, nil
}

// SelectOption selects the target option in HTMLSelectElement.
func (a *App) SelectOption(ctx context.Context, ui UIComponent, value string) error {
	if err := a.WaitForVisibleState(ctx, ui, true); err != nil {
		return err
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return err
	}
	return a.conn.Call(ctx, nil, "Tast.selectOption", selector, value)
}

// InputRange returns the range of valid value for range type input element.
func (a *App) InputRange(ctx context.Context, ui UIComponent) (*Range, error) {
	if err := a.WaitForVisibleState(ctx, ui, true); err != nil {
		return nil, err
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return nil, err
	}
	var r Range
	if err := a.conn.Call(ctx, &r, "Tast.getInputRange", selector); err != nil {
		return nil, errors.Wrapf(err, "failed to get input range of %v", ui.Name)
	}
	return &r, nil
}

// SetRangeInput set value of range input.
func (a *App) SetRangeInput(ctx context.Context, ui UIComponent, value int) error {
	if err := a.WaitForVisibleState(ctx, ui, true); err != nil {
		return err
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return err
	}
	if err := a.conn.Call(ctx, nil, "Tast.setInputValue", selector, value); err != nil {
		return errors.Wrapf(err, "failed to set range input %v to %v", ui.Name, value)
	}
	return nil
}
