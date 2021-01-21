/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.perappdensitytest;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.widget.Button;

/** Test Activity for arc.PerAppDensity test. */
public class ViewActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Hide action bar.
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.view_activity);

        View view = (View)findViewById(R.id.view);
        view.setOnClickListener(new View.OnClickListener() {
          @Override
          public void onClick(View v) {
            startActivity(new Intent(ViewActivity.this, SecondActivity.class));
          }
        });
    }
}
