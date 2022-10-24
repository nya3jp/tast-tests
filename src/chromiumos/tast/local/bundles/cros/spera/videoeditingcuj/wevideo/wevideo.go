// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wevideo implements WeVideo operations.
package wevideo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	dragTime       = 2 * time.Second  // Used for dragging.
	shortUITimeout = 3 * time.Second  // Used for situations where UI response might be faster.
	longUITimeout  = 30 * time.Second // Used for situations where UI response might be slow.
	retryTimes     = 3                // retryTimes is the maximum number of times the action will be retried.
)

var weVideoWebArea = nodewith.Name("WeVideo").Role(role.RootWebArea)
var beginningOfTheClip = nodewith.NameContaining("the beginning of the clip").ClassName("trim-btn")
var endingOfTheClip = nodewith.NameContaining("the ending of the clip").ClassName("trim-btn")

// Clip defines the struct related to WeVideo's clip.
type Clip struct {
	name       string
	startPoint coords.Point
	endPoint   coords.Point
}

// WeVideo defines the struct related to WeVideo web.
type WeVideo struct {
	conn       *chrome.Conn
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	uiHandler  cuj.UIActionHandler
	kb         *input.KeyboardEventWriter
	clips      map[string]Clip
	tabletMode bool
	br         *browser.Browser
}

// NewWeVideo creates an instance of WeVideo.
func NewWeVideo(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uiHandler cuj.UIActionHandler, tabletMode bool, br *browser.Browser) *WeVideo {
	return &WeVideo{
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		kb:         kb,
		uiHandler:  uiHandler,
		clips:      make(map[string]Clip),
		tabletMode: tabletMode,
		br:         br,
	}
}

// Open opens a WeVideo webpage on chrome browser.
func (w *WeVideo) Open() action.Action {
	return func(ctx context.Context) (err error) {
		w.conn, err = w.uiHandler.NewChromeTab(ctx, w.br, cuj.WeVideoURL, true)
		if err != nil {
			return errors.Wrap(err, "failed to connect to chrome")
		}
		return nil
	}
}

// Login logs in WeVideo.
func (w *WeVideo) Login(account string) action.Action {
	ui := w.ui
	loginRequired := func(ctx context.Context) error {
		if err := ui.Exists(weVideoWebArea)(ctx); err == nil {
			return errors.New("It has been loged in")
		}
		return nil
	}
	loginButton := nodewith.Name("Log in").Role(role.Button)
	loginReg := regexp.MustCompile(`(Login|Log in) to your account`)
	loginWebArea := nodewith.NameRegex(loginReg).Role(role.RootWebArea)
	googleLink := nodewith.Name("Log in with Google").Role(role.Link).Ancestor(loginWebArea)
	targetAccount := nodewith.Name(account).Role(role.StaticText)

	loginWithGoogle := uiauto.NamedCombine("login with Google",
		ui.DoDefaultUntil(googleLink, ui.WithTimeout(shortUITimeout).WaitUntilExists(targetAccount)),
		ui.LeftClickUntil(targetAccount, ui.WithTimeout(shortUITimeout).WaitUntilGone(targetAccount)),
	)
	// Sign up process.
	signUpWebArea := nodewith.Name("WeVideo Checkout").Role(role.RootWebArea)
	answer1 := nodewith.Name("Business / Marketing").Role(role.Button)
	answer2 := nodewith.Name("Product demos").Role(role.Button)
	createButton := nodewith.Name("Start creating!").Role(role.Button)
	signUp := uiauto.NamedCombine("sign up WeVideo",
		ui.LeftClick(answer1),
		ui.LeftClick(answer2),
		ui.LeftClick(createButton),
	)

	return uiauto.IfSuccessThen(loginRequired,
		// There is a bug in Wevideo login process, sometimes it needs to login twice with google account.
		// So add retry login here.
		uiauto.Retry(retryTimes, uiauto.NamedCombine("log in WeVideo",
			uiauto.IfSuccessThen(ui.Exists(loginButton), ui.DoDefault(loginButton)),
			ui.WaitUntilExists(loginWebArea),
			loginWithGoogle,
			// Sign up if there is a sign up page.
			uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(signUpWebArea), signUp),
			uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(loginWebArea), loginWithGoogle),
			ui.WaitUntilExists(weVideoWebArea),
		)))
}

// Create creates the new video editing.
func (w *WeVideo) Create() action.Action {
	promptWindow := nodewith.ClassName("Modal medium")
	closeButton := nodewith.Name("CLOSE").Role(role.Button).Ancestor(promptWindow)
	createNewRe := regexp.MustCompile("(?i)create new")
	createNewButton := nodewith.NameRegex(createNewRe).Ancestor(weVideoWebArea).First()
	videoText := nodewith.Name("Video").Role(role.StaticText).Ancestor(weVideoWebArea)
	titleRe := regexp.MustCompile("(?i)my video")
	titleText := nodewith.NameRegex(titleRe).Role(role.StaticText).Ancestor(weVideoWebArea)
	// The pop-up prompt window display time is not necessarily, so add retry to ensure that the window is closed.
	return uiauto.Retry(retryTimes, uiauto.NamedCombine("create the new video editing",
		// Close the pop-up prompt window.
		uiauto.IfSuccessThen(w.ui.WithTimeout(shortUITimeout).WaitUntilExists(closeButton), w.ui.LeftClick(closeButton)),
		w.ui.DoDefault(createNewButton),
		w.ui.DoDefault(videoText),
		w.ui.WithTimeout(longUITimeout).WaitUntilExists(titleText),
	))
}

// AddStockVideo adds stock video to expected track.
func (w *WeVideo) AddStockVideo(clipName, previousClipName, clipTime, expectedTrack string) action.Action {
	ui := w.ui
	searchVideo := nodewith.Name("Search videos").Role(role.TextField)
	stockMediaButton := nodewith.Name("Videos").Role(role.Button).HasClass("MuiListItem-button")
	tryItNowButton := nodewith.Name("TRY IT NOW").Role(role.Button)
	openStockMedia := uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilGone(searchVideo),
		// The pop-up prompt window display time is not necessarily, so add retry to ensure that the window is closed.
		uiauto.Retry(retryTimes, uiauto.NamedCombine("open stock media",
			w.closePromptWindow(),
			ui.LeftClick(stockMediaButton),
			uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(tryItNowButton), ui.LeftClick(tryItNowButton)),
			ui.WaitUntilExists(searchVideo),
		)))
	findVideo := uiauto.NamedCombine("find video",
		ui.LeftClick(searchVideo),
		ui.WaitUntilExists(searchVideo.Focused()),
		w.kb.AccelAction("Ctrl+A"),
		w.kb.TypeAction(clipName),
		w.kb.AccelAction("Enter"),
	)
	dragVideoToTrack := func(ctx context.Context) error {
		clipButton := nodewith.NameContaining(clipTime).Role(role.StaticText)
		// Finding video from WeVideo videos may take a long time to load.
		clipLocation, err := ui.WithTimeout(longUITimeout).Location(ctx, clipButton)
		if err != nil {
			return err
		}

		dragUpStart := clipLocation.CenterPoint()
		var dragUpEnd coords.Point
		if previousClipName != "" {
			dragUpEnd = w.clips[previousClipName].endPoint
		} else {
			expectedTrack := nodewith.Name(expectedTrack).Role(role.StaticText)
			trackLocation, err := ui.Location(ctx, expectedTrack)
			if err != nil {
				return err
			}
			playHead := nodewith.Name("Playhead").Role(role.GenericContainer)
			playHeadLocation, err := ui.Location(ctx, playHead)
			if err != nil {
				return err
			}
			dragUpEnd = coords.NewPoint(playHeadLocation.Right(), trackLocation.CenterY())
		}
		insertAndPush := nodewith.NameContaining("Insert and push").Role(role.StaticText)
		pc := pointer.NewMouse(w.tconn)
		defer pc.Close()
		testing.ContextLogf(ctx, "Drag video to track from %v to %v", dragUpStart, dragUpEnd)
		// Sometimes it fails to drag the video, so add a retry here.
		return uiauto.Retry(retryTimes, uiauto.Combine("drag video to track",
			pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragTime)),
			uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(insertAndPush), ui.LeftClick(insertAndPush)),
			ui.WithTimeout(shortUITimeout).WaitUntilExists(beginningOfTheClip),
		))(ctx)
	}
	addClip := func(ctx context.Context) error {
		startLocation, err := ui.Location(ctx, beginningOfTheClip)
		if err != nil {
			return err
		}
		endLocation, err := ui.Location(ctx, endingOfTheClip)
		if err != nil {
			return err
		}
		w.clips[clipName] = Clip{
			name:       clipName,
			startPoint: startLocation.CenterPoint(),
			endPoint:   endLocation.CenterPoint(),
		}
		return nil
	}
	return uiauto.NamedCombine(fmt.Sprintf("add stock video \"%s\"", clipName),
		openStockMedia,
		findVideo,
		dragVideoToTrack,
		addClip,
		ui.LeftClick(endingOfTheClip),
	)
}

// AddText adds static text to the expected track.
func (w *WeVideo) AddText(clipName, expectedTrack, text string) action.Action {
	ui := w.ui
	textButton := nodewith.Name("Text").Role(role.Button).HasClass("MuiListItem-button")
	// It removes the text info, so it can only capture "Basic text" node by classname.
	// The first one is "Basic text".
	basicText := nodewith.ClassName("ui-draggable-handle").Role(role.GenericContainer).First()
	dragTextToTrack := func(ctx context.Context) error {
		textLocation, err := ui.Location(ctx, basicText)
		if err != nil {
			return err
		}
		expectedTrack := nodewith.Name(expectedTrack).Role(role.StaticText)
		trackLocation, err := ui.Location(ctx, expectedTrack)
		if err != nil {
			return err
		}
		dragUpStart := textLocation.CenterPoint()
		dragUpEnd := coords.NewPoint(w.clips[clipName].startPoint.X, trackLocation.CenterY())
		pc := pointer.NewMouse(w.tconn)
		defer pc.Close()
		testing.ContextLogf(ctx, "Drag text to track from %v to %v", dragUpStart, dragUpEnd)
		return uiauto.Retry(retryTimes, uiauto.Combine("drag text to track",
			pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragTime)),
			ui.WithTimeout(shortUITimeout).WaitUntilExists(beginningOfTheClip),
		))(ctx)
	}
	sampleText := nodewith.Name("Sample text").Role(role.StaticText)
	saveButtonRe := regexp.MustCompile("(?i)save changes")
	saveButton := nodewith.NameRegex(saveButtonRe).Role(role.StaticText).Ancestor(weVideoWebArea)
	editTextProperties := uiauto.NamedCombine("edit text",
		w.closePromptWindow(),
		w.kb.TypeAction("e"), // Type e to edit text.
		ui.LeftClick(sampleText),
		w.kb.AccelAction("Ctrl+A"),
		w.kb.TypeAction(text),
		ui.DoDefaultUntil(saveButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(saveButton)),
	)
	return uiauto.NamedCombine(fmt.Sprintf("add text to clip \"%s\"", clipName),
		w.closePromptWindow(),
		ui.LeftClick(textButton),
		dragTextToTrack,
		editTextProperties,
	)
}

// AddTransition adds transition to the expected clip.
func (w *WeVideo) AddTransition(clipName string) action.Action {
	transitionButton := nodewith.Name("Transitions").Role(role.Button).HasClass("MuiListItem-button")
	transitionClip := nodewith.HasClass("clip-transition").Role(role.GenericContainer)
	dragTransitionToClip := func(ctx context.Context) error {
		crossFade := nodewith.Name("Cross fade").Role(role.StaticText)
		crossFadeLocation, err := w.ui.Location(ctx, crossFade)
		if err != nil {
			return err
		}
		dragUpStart := crossFadeLocation.CenterPoint()
		dragUpEnd := w.clips[clipName].startPoint
		pc := pointer.NewMouse(w.tconn)
		defer pc.Close()
		testing.ContextLogf(ctx, "Drag transition to clip from %v to %v", dragUpStart, dragUpEnd)
		return uiauto.Retry(retryTimes, uiauto.Combine("drag transition to clip",
			pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragTime)),
			// Check the transition is added.
			w.ui.WithTimeout(shortUITimeout).WaitUntilExists(transitionClip),
		))(ctx)

	}
	return uiauto.NamedCombine(fmt.Sprintf("add transition \"Cross fade\" to clip \"%s\"", clipName),
		w.closePromptWindow(),
		w.ui.LeftClick(transitionButton),
		dragTransitionToClip,
	)
}

// PlayVideo plays the edited video from the beginning of expected clip.
func (w *WeVideo) PlayVideo(clipName string) action.Action {
	return uiauto.NamedCombine("play the edited video from the beginning to the end",
		w.ui.MouseClickAtLocation(0, w.clips[clipName].startPoint),
		w.kb.AccelAction("Space"), // Press space to play video.
		w.waitUntilPlaying(shortUITimeout),
		w.waitUntilPaused(longUITimeout),
	)
}

// waitUntilPlaying waits until the edited video is playing.
func (w *WeVideo) waitUntilPlaying(timeout time.Duration) action.Action {
	const preparingText = "We're preparing your preview"
	var err error
	var startTime time.Time
	var startPlayTime, currentPlayTime string
	setStartPlayTime := func(ctx context.Context) error {
		startTime = time.Now()
		startPlayTime, err = w.currentTime(ctx)
		return err
	}
	setCurrentPlayTime := func(ctx context.Context) error {
		currentPlayTime, err = w.currentTime(ctx)
		return err
	}
	checkIsPlaying := func(ctx context.Context) error {
		clearAllButton := nodewith.Name("Clear All").Role(role.Button).Ancestor(weVideoWebArea)
		preparingDialog := nodewith.Name(preparingText).Role(role.StaticText).Ancestor(weVideoWebArea)
		if err := uiauto.Combine("clear popup",
			uiauto.IfSuccessThen(w.ui.Exists(clearAllButton), w.ui.LeftClick(clearAllButton)),
			w.ui.WaitUntilGone(preparingDialog),
		)(ctx); err != nil {
			return err
		}
		if currentPlayTime != startPlayTime {
			duration := time.Now().Sub(startTime)
			testing.ContextLogf(ctx, "Wait for %v to play video", duration)
			return nil
		}
		return errors.New("video is not playing")
	}
	return uiauto.NamedCombine("wait until playing",
		setStartPlayTime,
		w.ui.WithTimeout(timeout+longUITimeout).RetryUntil(setCurrentPlayTime, checkIsPlaying),
	)
}

// waitUntilPaused waits until the edited video is paused.
func (w *WeVideo) waitUntilPaused(timeout time.Duration) action.Action {
	var startTime time.Time
	var startPlayTime, currentPlayTime string
	getStartTime := func(ctx context.Context) error {
		startTime = time.Now()
		return nil
	}
	getPlayTime := func(ctx context.Context) (err error) {
		startPlayTime, err = w.currentTime(ctx)
		if err != nil {
			return err
		}
		// Sleep for a second to check whether the video is paused.
		if err := uiauto.Sleep(time.Second)(ctx); err != nil {
			return err
		}
		currentPlayTime, err = w.currentTime(ctx)
		return err
	}
	checkIsPaused := func(ctx context.Context) error {
		if currentPlayTime == startPlayTime {
			duration := time.Now().Sub(startTime)
			testing.ContextLogf(ctx, "Wait for %v to pause video", duration)
			return nil
		}
		return errors.New("video is not paused")
	}
	return uiauto.NamedCombine("wait until paused",
		getStartTime,
		w.ui.WithTimeout(timeout).RetryUntil(getPlayTime, checkIsPaused),
	)
}

// currentTime gets the current playing time (in string) of the edited video.
func (w *WeVideo) currentTime(ctx context.Context) (string, error) {
	timeNodeRe := regexp.MustCompile("^(\\d+):(\\d+):(\\d+) / (\\d+):(\\d+):(\\d+)$")
	timeNodeText := nodewith.NameRegex(timeNodeRe).Role(role.StaticText).Ancestor(weVideoWebArea)
	if err := w.ui.Exists(timeNodeText)(ctx); err == nil {
		info, err := w.ui.Info(ctx, timeNodeText)
		if err != nil {
			return "", err
		}
		currentTime := strings.Split(info.Name, " ")[0]
		return currentTime, nil
	}

	playHead := nodewith.Name("Playhead").Role(role.GenericContainer).Ancestor(weVideoWebArea)
	timeNodeText = nodewith.NameRegex(regexp.MustCompile("^(\\d+):(\\d+):(\\d+)$")).Role(role.StaticText).Ancestor(playHead)
	info, err := w.ui.Info(ctx, timeNodeText)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}

// closePromptWindow closes the pop-up prompt window.
func (w *WeVideo) closePromptWindow() action.Action {
	promptWindow := nodewith.NameContaining("Intercom Live Chat").Role(role.Dialog)
	closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(promptWindow).First()
	closeDialog := uiauto.NamedAction("close prompt window",
		w.ui.LeftClickUntil(closeButton, w.ui.WithTimeout(shortUITimeout).WaitUntilGone(closeButton)))
	return uiauto.IfSuccessThen(w.ui.WithTimeout(shortUITimeout).WaitUntilExists(closeButton), closeDialog)
}

// Close closes the WeVideo page.
func (w *WeVideo) Close(ctx context.Context) {
	if w.conn == nil {
		return
	}
	w.conn.CloseTarget(ctx)
	w.conn.Close()
}
