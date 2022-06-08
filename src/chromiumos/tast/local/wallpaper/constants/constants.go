// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package constants contains values used across wallpaper tests.
package constants

import (
	"image/color"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// FillButton is a finder for the "Fill" toggle button node.
var FillButton = nodewith.Name("Fill").Role(role.ToggleButton)

// CenterButton is a finder for the "Center" toggle button node.
var CenterButton = nodewith.Name("Center").Role(role.ToggleButton)

// ChangeDailyButton is a finder for the "Change Daily" toggle button node.
var ChangeDailyButton = nodewith.Name("Change wallpaper image daily").Role(role.ToggleButton)

// GooglePhotosWallpaperAlbumsButton is a finder for the Google Photos "Albums" toggle button node.
var GooglePhotosWallpaperAlbumsButton = nodewith.Name("Albums").Role(role.ToggleButton)

// GooglePhotosWallpaperAlbum is the name of an album in the GooglePhotosWallpaperCollection.
const GooglePhotosWallpaperAlbum = "Album 01"

// GooglePhotosWallpaperCollection is the name of the Google Photos wallpaper collection.
const GooglePhotosWallpaperCollection = "Google Photos"

// GooglePhotosWallpaperPhoto is the name of a photo in the GooglePhotosWallpaperAlbum.
const GooglePhotosWallpaperPhoto = "Photo 01"

// GooglePhotosWallpaperColor is the color of the GooglePhotosWallpaperPhoto.
var GooglePhotosWallpaperColor = color.RGBA{0, 0, 255, 255}

// RefreshButton is a finder for the "Refresh" button node.
var RefreshButton = nodewith.Name("Refresh the current wallpaper image").Role(role.Button)

// SolidColorsCollection is the name of a wallpaper collection of solid colors.
const SolidColorsCollection = "Solid colors"

// ElementCollection is the name of a wallpaper collection of elements.
const ElementCollection = "Element"

// DarkElementImage and LightElementImage are two images in Element collection.
const (
	DarkElementImage  = "Wind Dark Digital Art by Rutger Paulusse"
	LightElementImage = "Wind Light Digital Art by Rutger Paulusse"
)

// YellowWallpaperName is the name of a solid yellow wallpaper in the solid colors collection.
const YellowWallpaperName = "Yellow"

// YellowWallpaperColor is the color of the Solid Colors / Yellow wallpaper image.
var YellowWallpaperColor = color.RGBA{255, 235, 60, 255}

// LocalWallpaperCollection is the wallpaper collection of images stored in the device's Downloads folder.
const LocalWallpaperCollection = "My Images"

// LocalWallpaperFilename is the filename of the image in the Downloads folder.
const LocalWallpaperFilename = "set_local_wallpaper_light_pink_20210929.jpg"

// LocalWallpaperColor is the color of LocalWallpaperFilename.
var LocalWallpaperColor = color.RGBA{255, 203, 198, 255}

// WhiteWallpaperName is the name of a solid white wallpaper in the solid colors collection.
const WhiteWallpaperName = "White"
