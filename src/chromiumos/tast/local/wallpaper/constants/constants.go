// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package constants contains values used across wallpaper tests.
package constants

import "image/color"

// SolidColorsCollection is the name of a wallpaper collection of solid colors.
const SolidColorsCollection = "Solid colors"

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
