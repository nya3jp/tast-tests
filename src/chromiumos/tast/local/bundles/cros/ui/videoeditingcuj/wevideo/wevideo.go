// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wevideo implements WeVideo operations.
package wevideo

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
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
}

// NewWeVideo creates an instance of WeVideo.
func NewWeVideo(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uiHandler cuj.UIActionHandler, tabletMode bool) *WeVideo {
	clips := make(map[string]Clip)
	return &WeVideo{
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		kb:         kb,
		uiHandler:  uiHandler,
		clips:      clips,
		tabletMode: tabletMode,
	}
}

// Open opens a WeVideo webpage on chrome browser.
func (w *WeVideo) Open(cr *chrome.Chrome) action.Action {
	return func(ctx context.Context) (err error) {
		w.conn, err = cr.NewConn(ctx, cuj.WeVideoURL)
		if err != nil {
			return errors.Wrap(err, "failed to connect to chrome")
		}
		return nil
	}
}

// Login logs in WeVideo.
func (w *WeVideo) Login(account string) action.Action {
	const actionName = "log in WeVideo"
	ui := w.ui
	loginRequired := func(ctx context.Context) error {
		if err := ui.Exists(weVideoWebArea)(ctx); err == nil {
			return errors.New("It has been loged in")
		}
		return nil
	}
	loginButton := nodewith.Name("Log in").Role(role.Button)
	loginWebArea := nodewith.Name("Log in to your account").Role(role.RootWebArea)
	googleLink := nodewith.Name("Log in with Google").Role(role.Link).Ancestor(loginWebArea)
	targetAccount := nodewith.Name(account).Role(role.StaticText)

	loginWithGoogle := uiauto.NamedAction("login with Google",
		uiauto.Combine("login with Google",
			ui.LeftClickUntil(googleLink, ui.WithTimeout(shortUITimeout).WaitUntilExists(targetAccount)),
			ui.LeftClickUntil(targetAccount, ui.WithTimeout(shortUITimeout).WaitUntilGone(targetAccount)),
		))
	// Sign up process.
	signUpWebArea := nodewith.Name("WeVideo Checkout").Role(role.RootWebArea)
	answer1 := nodewith.Name("Business / Marketing").Role(role.Button)
	answer2 := nodewith.Name("Product demos").Role(role.Button)
	createButton := nodewith.Name("Start creating!").Role(role.Button)
	signUp := uiauto.NamedAction("sign up WeVideo",
		uiauto.Combine("sign up WeVideo",
			ui.LeftClick(answer1),
			ui.LeftClick(answer2),
			ui.LeftClick(createButton),
		))

	return uiauto.IfSuccessThen(loginRequired,
		uiauto.NamedAction(actionName,
			// There is a bug in Wevideo login process, sometimes it needs to login twice with google account.
			// So add retry login here.
			uiauto.Retry(3,
				uiauto.Combine(actionName,
					uiauto.IfSuccessThen(ui.Exists(loginButton), ui.LeftClick(loginButton)),
					ui.WaitUntilExists(loginWebArea),
					loginWithGoogle,
					// Sign up if there is a sign up page.
					uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(signUpWebArea), signUp),
					uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(loginWebArea), loginWithGoogle),
					ui.WaitUntilExists(weVideoWebArea),
				))))
}

// Create creates the new video editing.
func (w *WeVideo) Create() action.Action {
	const actionName = "create the new video editing"
	promptWindow := nodewith.ClassName("Modal medium")
	closeButton := nodewith.Name("CLOSE").Role(role.Button).Ancestor(promptWindow)
	createNewButton := nodewith.NameContaining("CREATE NEW").Role(role.Button).Ancestor(weVideoWebArea)
	videoText := nodewith.Name("Video").Role(role.StaticText).Ancestor(weVideoWebArea)
	titleText := nodewith.Name("MY VIDEO").Role(role.StaticText).Ancestor(weVideoWebArea)
	return uiauto.NamedAction(actionName,
		uiauto.Retry(3, uiauto.Combine(actionName,
			// Close the pop-up prompt window.
			uiauto.IfSuccessThen(w.ui.WithTimeout(shortUITimeout).WaitUntilExists(closeButton), w.ui.LeftClick(closeButton)),
			w.ui.LeftClick(createNewButton),
			w.ui.WaitForLocation(videoText),
			w.ui.LeftClick(videoText),
			w.ui.WithTimeout(longUITimeout).WaitUntilExists(titleText),
		)))
}

// AddStockVideo adds stock video to expected track.
func (w *WeVideo) AddStockVideo(clipName, clipTime, expectedTrack string) action.Action {
	actionName := fmt.Sprintf("add stock video \"%s\"", clipName)
	ui := w.ui
	searchVideo := nodewith.Name("Search videos").Role(role.TextField)
	stockMediaButton := nodewith.Name("Videos").Role(role.Button).HasClass("MuiListItem-button")
	tryItNowButton := nodewith.Name("TRY IT NOW").Role(role.Button)
	openStockMedia := uiauto.IfSuccessThen(ui.Gone(searchVideo),
		uiauto.NamedAction("open stock media",
			uiauto.Retry(3, uiauto.Combine("open stock media",
				w.closePromptWindow(),
				ui.LeftClick(stockMediaButton),
				uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(tryItNowButton), ui.LeftClick(tryItNowButton)),
				ui.WaitUntilExists(searchVideo),
			))))
	findVideo := uiauto.NamedAction("find video",
		uiauto.Combine("find video",
			ui.LeftClick(searchVideo),
			ui.WaitUntilExists(searchVideo.Focused()),
			w.kb.AccelAction("Ctrl+A"),
			w.kb.TypeAction(clipName),
			w.kb.AccelAction("Enter"),
		))
	dragVideoToTrack := func(ctx context.Context) error {
		clipButton := nodewith.NameContaining(clipTime).Role(role.StaticText)
		// Finding video from WeVideo videos may take a long time to load.
		clipLocation, err := ui.WithTimeout(longUITimeout).Location(ctx, clipButton)
		if err != nil {
			return err
		}
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
		dragUpStart := clipLocation.CenterPoint()
		dragUpEnd := coords.NewPoint(playHeadLocation.Right(), trackLocation.CenterY())
		insertAndPush := nodewith.NameContaining("Insert and push").Role(role.StaticText)
		pc := pointer.NewMouse(w.tconn)
		defer pc.Close()
		testing.ContextLogf(ctx, "Drag video to track from %v to %v", dragUpStart, dragUpEnd)
		return uiauto.Retry(3, uiauto.Combine("drag video to track",
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
		clip := Clip{
			name:       clipName,
			startPoint: startLocation.CenterPoint(),
			endPoint:   endLocation.CenterPoint(),
		}
		w.clips[clipName] = clip
		return nil
	}
	return uiauto.NamedAction(actionName,
		uiauto.Combine(actionName,
			openStockMedia,
			findVideo,
			dragVideoToTrack,
			addClip,
			ui.LeftClick(endingOfTheClip),
		))
}

// AddText adds static text to the expected track.
func (w *WeVideo) AddText(clipName, expectedTrack, text string) action.Action {
	actionName := fmt.Sprintf("add text to clip \"%s\"", clipName)
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
		return uiauto.Retry(3, uiauto.Combine("drag text to track",
			pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragTime)),
			ui.WithTimeout(shortUITimeout).WaitUntilExists(beginningOfTheClip),
		))(ctx)
	}
	sampleText := nodewith.Name("Sample text").Role(role.StaticText)
	saveButton := nodewith.Name("SAVE CHANGES").Role(role.StaticText)
	editTextProperties := func(ctx context.Context) error {
		var height int64
		if err := w.tconn.Eval(ctx, "window.screen.height", &height); err != nil {
			return errors.Wrap(err, "failed to retrieve screen height")
		}
		scrollLowScreenHeight := func(scrollAction action.Action) action.Action {
			return func(ctx context.Context) error {
				if !w.tabletMode && height <= 750 {
					testing.ContextLogf(ctx, "The screen height(%d) is not enough, scrolling the page", height)
					return scrollAction(ctx)
				}
				return nil
			}
		}
		return uiauto.NamedAction("edit text",
			uiauto.Combine("edit text",
				w.closePromptWindow(),
				ui.DoubleClick(beginningOfTheClip),
				ui.LeftClick(sampleText),
				w.kb.AccelAction("Ctrl+A"),
				w.kb.TypeAction(text),
				// Some DUTs have too little screen height to display the save button.
				// Swipe down to slide the page down to reveal the save button.
				// There are two scroll bars in the text properties, so scroll twice here.
				scrollLowScreenHeight(w.uiHandler.SwipeDown()),
				scrollLowScreenHeight(w.uiHandler.SwipeDown()),
				ui.LeftClickUntil(saveButton, ui.Gone(saveButton)),
				// After saving, scroll back to the top.
				scrollLowScreenHeight(w.uiHandler.SwipeUp()),
			))(ctx)
	}

	return uiauto.NamedAction(actionName,
		uiauto.Combine(actionName,
			w.closePromptWindow(),
			ui.LeftClick(textButton),
			dragTextToTrack,
			editTextProperties,
		))
}

// AddTransition adds transition to the expected clip.
func (w *WeVideo) AddTransition(clipName string) action.Action {
	actionName := fmt.Sprintf("add transition \"Cross fade\" to clip \"%s\"", clipName)
	transitionButton := nodewith.Name("Transitions").Role(role.Button).HasClass("MuiListItem-button")
	transitionClip := nodewith.ClassName("transition-icon").Role(role.GenericContainer)
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
		return uiauto.Retry(3, uiauto.Combine("drag transition to clip",
			pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragTime)),
			// Check the transition is added.
			w.ui.WaitUntilExists(transitionClip),
		))(ctx)

	}
	return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
		w.closePromptWindow(),
		w.ui.LeftClick(transitionButton),
		dragTransitionToClip,
	))
}

// PlayVideo plays the edited video from the beginning of expected clip.
func (w *WeVideo) PlayVideo(clipName string) action.Action {
	const actionName = "play the edited video from the beginning to the end"
	playButton := nodewith.NameContaining("Play the video").Role(role.GenericContainer)
	return uiauto.NamedAction(actionName,
		uiauto.Combine(actionName,
			w.ui.MouseClickAtLocation(0, w.clips[clipName].startPoint),
			w.ui.LeftClick(playButton),
			w.waitUntilPlaying(shortUITimeout),
			w.waitUntilPaused(longUITimeout),
		))
}

// waitUntilPlaying waits until the edited video is playing.
func (w *WeVideo) waitUntilPlaying(timeout time.Duration) action.Action {
	const (
		actionName    = "wait until playing"
		preparingText = "We're preparing your preview"
	)
	var startTime time.Time
	var startPlayTime, currentPlayTime string
	getStartPlayTime := func(ctx context.Context) (err error) {
		startTime = time.Now()
		startPlayTime, err = w.getCurrentTime(ctx)
		return err
	}
	getCurrentPlayTime := func(ctx context.Context) (err error) {
		currentPlayTime, err = w.getCurrentTime(ctx)
		return err
	}
	checkIsPlaying := func(ctx context.Context) error {
		clearAllButton := nodewith.Name("Clear All").Role(role.Button).Ancestor(weVideoWebArea)
		preparingDialog := nodewith.Name(preparingText).Role(role.StaticText).Ancestor(weVideoWebArea)
		if err := uiauto.Combine("clear popup",
			uiauto.IfSuccessThen(w.ui.Exists(clearAllButton), w.ui.LeftClick(clearAllButton)),
			uiauto.IfSuccessThen(w.ui.Exists(preparingDialog), w.ui.WaitUntilGone(preparingDialog)),
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
	return uiauto.NamedAction(actionName,
		uiauto.Combine(actionName,
			getStartPlayTime,
			w.ui.WithTimeout(timeout+longUITimeout).RetryUntil(getCurrentPlayTime, checkIsPlaying),
		))
}

// waitUntilPaused waits until the edited video is paused.
func (w *WeVideo) waitUntilPaused(timeout time.Duration) action.Action {
	const actionName = "wait until paused"
	var startTime time.Time
	var startPlayTime, currentPlayTime string
	getStartTime := func(ctx context.Context) error {
		startTime = time.Now()
		return nil
	}
	getPlayTime := func(ctx context.Context) (err error) {
		startPlayTime, err = w.getCurrentTime(ctx)
		if err != nil {
			return err
		}
		// Sleep for a second to check whether the video is paused.
		if err := uiauto.Sleep(time.Second)(ctx); err != nil {
			return err
		}
		currentPlayTime, err = w.getCurrentTime(ctx)
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
	return uiauto.NamedAction(actionName,
		uiauto.Combine(actionName,
			getStartTime,
			w.ui.WithTimeout(timeout).RetryUntil(getPlayTime, checkIsPaused),
		))
}

// getCurrentTime gets the current playing time (in string) of the edited video.
func (w *WeVideo) getCurrentTime(ctx context.Context) (timeInString string, err error) {
	playHead := nodewith.Name("Playhead").Role(role.GenericContainer).Ancestor(weVideoWebArea)
	timeNode := nodewith.NameRegex(regexp.MustCompile("^(\\d+):(\\d+):(\\d+)$")).Role(role.StaticText).Ancestor(playHead)
	info, err := w.ui.Info(ctx, timeNode)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}

// closePromptWindow closes the pop-up prompt window.
func (w *WeVideo) closePromptWindow() action.Action {
	promptWindow := nodewith.NameContaining("Intercom Live Chat").Role(role.RootWebArea)
	closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(promptWindow)
	return uiauto.IfSuccessThen(w.ui.WithTimeout(shortUITimeout).WaitUntilExists(closeButton), w.ui.LeftClick(closeButton))
}

// Close closes the WeVideo page.
func (w *WeVideo) Close(ctx context.Context) {
	if w.conn == nil {
		return
	}
	w.conn.CloseTarget(ctx)
	w.conn.Close()
}
