// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.appposition;

import android.app.ActivityOptions;
import android.content.Intent;
import android.graphics.Rect;
import android.graphics.RectF;
import android.graphics.drawable.ColorDrawable;
import android.hardware.display.DisplayManager;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.view.Display;
import android.view.DisplayInfo;
import android.view.View;

import com.google.android.chromeos.activity.ChromeOsTaskManagement;

public class BackgroundActivity extends BaseActivity {
    private static final String TAG = "BackgroundActivity";

    private static final RectF sRelativeBounds = new RectF(0.5f, 0.2f, 0.8f, 0.8f);

    private final Rect mTmpBounds = new Rect();

    @Override
    public void onCreate(Bundle savedStateInstance) {
        super.onCreate(savedStateInstance);
        mImage.setImageDrawable(new ColorDrawable(getColor(R.color.background)));

        mTaskManagement.setTaskWindowState(ChromeOsTaskManagement.WINDOW_STATE_MAXIMIZED);
        getWindow()
                .getDecorView()
                .setSystemUiVisibility(
                        View.SYSTEM_UI_FLAG_IMMERSIVE
                                // Set the content to appear under the system bars so that the
                                // content doesn't resize when the system bars hide and show.
                                | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                                | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                                | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                                // Hide the nav bar and status bar
                                | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                                | View.SYSTEM_UI_FLAG_FULLSCREEN);
    }

    @Override
    public void onStart() {
        super.onStart();
        new Handler(Looper.getMainLooper()).post(this::launchMainActivity);
    }

    private void launchMainActivity() {
        final Intent intent = new Intent(this, MainActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);

        calculateBounds(Display.DEFAULT_DISPLAY, mTmpBounds);
        ActivityOptions options =
                ActivityOptions.makeBasic()
                        .setLaunchBounds(mTmpBounds)
                        .setLaunchDisplayId(Display.DEFAULT_DISPLAY);
        Log.i(TAG, "Launching into bounds " + mTmpBounds);
        startActivity(intent, options.toBundle());
    }

    private void calculateBounds(int displayId, Rect outBounds) {
        final DisplayManager dm = getSystemService(DisplayManager.class);
        final Display display = dm.getDisplay(displayId);
        if (display == null) {
            throw new IllegalStateException("Can't find display " + displayId);
        }
        final DisplayInfo displayInfo = new DisplayInfo();
        display.getDisplayInfo(displayInfo);
        Log.i(
                TAG,
                "Display "
                        + displayId
                        + " app bounds "
                        + displayInfo.appWidth
                        + "x"
                        + displayInfo.appHeight);

        outBounds.set(
                (int) (displayInfo.appWidth * sRelativeBounds.left),
                (int) (displayInfo.appHeight * sRelativeBounds.top),
                (int) (displayInfo.appWidth * sRelativeBounds.right),
                (int) (displayInfo.appHeight * sRelativeBounds.bottom));
    }
}
