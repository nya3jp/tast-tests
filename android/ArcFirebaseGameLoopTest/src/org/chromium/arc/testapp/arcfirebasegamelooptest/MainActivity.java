/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcfirebasegamelooptest;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.os.Bundle;

import java.util.Timer;
import java.util.TimerTask;

public class MainActivity extends Activity {
    int ticks = 0;
    boolean started = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        DrawView drawView = new DrawView(this);
        drawView.setBackgroundColor(Color.WHITE);
        setContentView(drawView);

        Intent launchIntent = getIntent();
        if(launchIntent.getAction().equals("com.google.intent.action.TEST_LOOP")) {
            started = true;
        }

        new Timer().scheduleAtFixedRate(new TimerTask() {
            @Override
            public void run() {
                // Wait until started.
                if(!started) {
                    return;
                }

                drawView.Move(.2f, .2f);
                drawView.invalidate();
                ticks++;
                if (ticks > 1000){
                    finish();
                }
            }
        }, 0, 1000/60);
    }
}
