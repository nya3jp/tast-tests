/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.pictureinpicturevideo;

/** Test Activity for the PIP Video Tast Test. */
public class VideoActivityWithRedSquare extends VideoActivity {

    @Override
    protected int getLayoutResID() {
        return R.layout.video_activity_with_red_square;
    }

    @Override
    protected int getTestVideoResID() {
        return R.id.testvideobeneathredsquare;
    }
}
