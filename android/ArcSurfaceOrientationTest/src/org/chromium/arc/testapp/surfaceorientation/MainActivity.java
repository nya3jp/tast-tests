/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.surfaceorientation;

import android.app.Activity;
import android.os.Bundle;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.os.Handler;
import android.util.Log;
import android.view.Surface;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.Window;

public class MainActivity extends Activity {

    private int mTransform;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.activity_main);

        Intent intent = getIntent();
        mTransform = intent.getIntExtra("transform", 0);

        final SurfaceView view = (SurfaceView) findViewById(R.id.surfaceView);
        final Handler handler = new Handler();
        final Runnable refresh = new Runnable() {
            @Override
            public void run() {
                renderToSurface(view.getHolder());
                // TODO: Does the render to surface thing need to be called multiple times?
                // handler.postDelayed(this, REFRESH_RATE_MS);
            }
        };

        view.getHolder().addCallback(new SurfaceHolder.Callback() {
            @Override
            public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {}
            @Override
            public void surfaceCreated(SurfaceHolder holder) {
                handler.post(refresh);
            }
            @Override
            public void surfaceDestroyed(SurfaceHolder holder) {
                handler.removeCallbacksAndMessages(null);
            }
        });
    }

    private void renderToSurface(SurfaceHolder holder) {
        nativeRenderToSurface(holder.getSurface(), mTransform);
    }

    private native void nativeRenderToSurface(Surface surface, int transform);

    // Used to load the 'native-lib' library on application startup.
    static {
        System.loadLibrary("arcsurfacerotationtest_jni");
    }
}