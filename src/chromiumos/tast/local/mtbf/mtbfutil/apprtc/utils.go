// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apprtc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

const (
	appRtcURL             = "https://apprtc.appspot.com/?VSC=VP8"
	roomShareLinkElement  = "#sharing-div"
	roomNameInputElement  = "#room-id-input"
	roomJoinButtonElement = "#join-button"
	roomFullElement       = "body > div"
)

func roomSelfOnly(ctx context.Context, conn *chrome.Conn) bool {
	expr := fmt.Sprintf(`document.querySelector("%s").innerText.includes("Waiting for someone to join this room")`, roomShareLinkElement)
	exist := ""
	conn.Eval(ctx, expr, &exist)
	return exist == "active"
}

func roomFull(ctx context.Context, conn *chrome.Conn) bool {
	expr := fmt.Sprintf(`document.querySelector("%s").innerText.includes("Sorry, this room is full.")`, roomFullElement)
	exist := false
	conn.Eval(ctx, expr, &exist)
	return exist
}

// JoinRtcRoom joins an AppRtc chatroom
func JoinRtcRoom(ctx context.Context, cr *chrome.Chrome, roomName string) error {
	conn, err := mtbfchrome.NewConn(ctx, cr, appRtcURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	expr := fmt.Sprintf("document.querySelector('%s').value = '%s'", roomNameInputElement, roomName)
	if err := conn.EvalPromise(ctx, expr, nil); err != nil {
		return mtbferrors.New(mtbferrors.ChromeExeJs, err, expr)
	}

	testing.Sleep(ctx, time.Second*2)

	if err := dom.ClickElement(ctx, conn, roomJoinButtonElement); err != nil {
		return err
	}

	if err := dom.WaitForDocumentReady(ctx, conn); err != nil {
		return mtbferrors.New(mtbferrors.VideoDocLoad, err)
	}

	testing.Sleep(ctx, time.Second*2)

	if roomSelfOnly(ctx, conn) {
		return mtbferrors.New(mtbferrors.VideoRoomEmpty, nil)
	}

	if roomFull(ctx, conn) {
		return mtbferrors.New(mtbferrors.VideoRoomFull, nil)
	}

	return nil
}
