/*
 * Copyright (C) 2019 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package org.chromium.arc.testapp.arcblackflashtest;

import static com.google.android.chromeos.activity.ChromeOsTaskManagement.WINDOW_STATE_APP_MANAGED;
import static com.google.android.chromeos.activity.ChromeOsTaskManagement.WINDOW_STATE_MAXIMIZED;
import static com.google.android.chromeos.activity.ChromeOsTaskManagement.WINDOW_STATE_NORMAL;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;

import com.google.android.chromeos.activity.ChromeOsTaskManagement;

/**
 * Test Activity that shows a black flash when maximized. The arc.BlackFlash test launches this app
 * in maximized state, restores it and maximizes it to check if blackflashes appear during those
 * state transitions.
 */
public class MainActivity extends Activity {

    // Note if we block the thread for more than 5 seconds, the Framework can throw ANR.
    private final int BLACK_FLASH_DURATION_MS = 3000;
    private final int WINDOW_STATE_INVALID = -1;

    private boolean mRestarted = false;
    private int mPrevWindowState = WINDOW_STATE_INVALID;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        if (savedInstanceState != null) {
            mRestarted = savedInstanceState.getBoolean("Restarted");
            mPrevWindowState = savedInstanceState.getInt("PrevWindowState");
        }

        final ChromeOsTaskManagement chromeOsTaskManagement =
                new ChromeOsTaskManagement(1 /* clientLibVersion */, this);
        final int windowState = chromeOsTaskManagement.getTaskWindowState().getWindowState();
        final boolean isMaximized =
                (windowState & ~WINDOW_STATE_APP_MANAGED) == WINDOW_STATE_MAXIMIZED;

        if (mRestarted && isMaximized) {
            try {
                // We need to block the UI thread to show black flashes.
                Thread.sleep(BLACK_FLASH_DURATION_MS);
            } catch (InterruptedException e) {
                Log.e("BlackFlashApp", e.toString());
            }
        }

        if (mPrevWindowState == WINDOW_STATE_NORMAL && isMaximized) {
            setContentView(R.layout.maximized);
        } else {
            setContentView(R.layout.main_activity);
        }

        mRestarted = true;
        mPrevWindowState = windowState;
    }

    @Override
    protected void onSaveInstanceState(Bundle savedInstanceState) {
        super.onSaveInstanceState(savedInstanceState);
        savedInstanceState.putBoolean("Restarted", mRestarted);
        savedInstanceState.putInt("PrevWindowState", mPrevWindowState);
    }
}
