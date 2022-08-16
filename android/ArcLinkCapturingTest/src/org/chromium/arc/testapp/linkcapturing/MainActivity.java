/*
 * Copyright 2022 The ChromiumOS Authors.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.linkcapturing;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;

/**
 * Main activity which responds to launcher Intents.
 *
 * <p>Shows a button which launches a link intent when clicked.
 */
public class MainActivity extends Activity {

    /** Intents sent to this URL can be handled by {@link LinkActivity}. */
    private static final String APP_URL = "http://127.0.0.1:8000/link_capturing/app/app_index.html";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        findViewById(R.id.link_action)
                .setOnClickListener(
                        view -> {
                            Intent intent = new Intent(Intent.ACTION_VIEW);
                            intent.setData(Uri.parse(APP_URL));
                            startActivity(intent);
                        });
    }
}
