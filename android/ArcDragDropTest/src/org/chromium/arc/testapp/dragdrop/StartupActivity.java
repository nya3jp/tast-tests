/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.dragdrop;

import android.app.Activity;
import android.app.ActivityOptions;
import android.content.Intent;
import android.graphics.Rect;
import android.os.Bundle;

/**
 * Activity which invokes DragDropActivity in preferred CSS pixel size.
 *
 * DragDropActivity needs to be launched in the specific position and the size, because Tast
 * injects mouse events which perform the drag and drop operation at the specific position
 * in the screen.
 *
 * The coordination of the recorded events is in CSS pixel, so the Activity's position and size need
 * to be specified in CSS pixel, too.
 *
 * StartupActivity receives the device scale factor in Intent extras, then dynamically specifies the
 * device pixel size of DragDropActivity which is calculated from CSS pixel size.
 *
 */
public class StartupActivity extends Activity {
    /**
     * Preferred activity width for DragDropActivity in CSS pixel.
     */
    private static final int ACTIVITY_CSS_PIXEL_WIDTH = 300;

    /**
     * Preferred activity height for DragDropActivity in CSS pixel.
     */
    private static final int ACTIVITY_CSS_PIXEL_HEIGHT = 300;

    /**
     * Ratio device pixel : CSS pixel.
     */
    private static final String EXTRA_DEVICE_SCALE_FACTOR = "DEVICE_SCALE_FACTOR";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        final double deviceScaleFactor = getIntent().getFloatExtra(EXTRA_DEVICE_SCALE_FACTOR, 1.0f);

        final Intent intent = new Intent(this, DragDropActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_LAUNCH_ADJACENT);
        final ActivityOptions options = ActivityOptions.makeBasic().setLaunchBounds(
                new Rect(0, 0, (int) (ACTIVITY_CSS_PIXEL_WIDTH * deviceScaleFactor),
                        (int) (ACTIVITY_CSS_PIXEL_HEIGHT * deviceScaleFactor)));
        startActivity(intent, options.toBundle());
        finish();
    }
}
