// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Print,
		Desc: "Check that ARC printing is working properly",
		Contacts: []string{
			"bmgordon@google.com",
			"jschettler@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcPrintTest.apk"},
		Pre:          arc.Booted(),
	})
}

func Print(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcPrintTest.apk"
		pkgName      = "org.chromium.arc.testapp.print"
		activityName = "MainActivity"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing ArcPrintTest app")
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install ArcPrintTest app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	// defer act.Close()

	s.Log("Starting MainActivity")
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start MainActivity: ", err)
	}

	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get UI root: ", err)
	}
	defer root.Release(ctx)

	// Find the destination list.
	params := ui.FindParams{
		Role: ui.RoleTypePopUpButton,
	}
	destList, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find destination list: ", err)
	}
	defer destList.Release(ctx)

	s.Log("Name: ", destList.Name)
	s.Log("ClassName: ", destList.ClassName)
	s.Log("Role: ", destList.Role)

	s.Log("x: ", destList.Location.Left+destList.Location.Width/2)
	s.Log("y: ", destList.Location.Top+destList.Location.Height/2)
	s.Log("Left: ", destList.Location.Left)
	s.Log("Top: ", destList.Location.Top)
	s.Log("Width: ", destList.Location.Width)
	s.Log("Height: ", destList.Location.Height)

	if err := destList.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click destination list: ", err)
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to wait for destination list to open: ", err)
	}

	if info, err := ui.RootDebugInfo(ctx, tconn); err == nil {
		s.Log(info)
	}

	params = ui.FindParams{
		Name: "See more…",
		Role: ui.RoleTypeMenuListOption,
	}
	seeMore, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find See more...: ", err)
	}
	defer seeMore.Release(ctx)

	s.Log("Name: ", seeMore.Name)
	s.Log("ClassName: ", seeMore.ClassName)
	s.Log("Role: ", seeMore.Role)

	s.Log("x: ", seeMore.Location.Left+seeMore.Location.Width/2)
	s.Log("y: ", seeMore.Location.Top+seeMore.Location.Height/2)
	s.Log("Left: ", seeMore.Location.Left)
	s.Log("Top: ", seeMore.Location.Top)
	s.Log("Width: ", seeMore.Location.Width)
	s.Log("Height: ", seeMore.Location.Height)

	if err := seeMore.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click See more...: ", err)
	}
}
