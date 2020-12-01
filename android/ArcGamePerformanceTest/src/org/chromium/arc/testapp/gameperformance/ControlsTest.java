/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

/**
 * Tests that verifies how many UI controls can be handled to keep FPS close to device refresh rate.
 * As a test UI control ImageView with an infinite animation is chosen. The animation has refresh
 * rate ~67Hz that forces all devices to refresh UI at highest possible rate.
 */
public class ControlsTest extends BaseTest {
    public ControlsTest(GamePerformanceActivity activity) {
        super(activity);
    }

    public CustomControlView getView() {
        return getActivity().getControlView();
    }

    @Override
    protected void initProbePass(int probe) {
        try {
            getActivity().attachControlView();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return;
        }
        initUnits(probe * getUnitScale());
    }

    @Override
    protected void freeProbePass() {}

    @Override
    public String getName() {
        return "control_count";
    }

    @Override
    public String getUnitName() {
        return "controls";
    }

    @Override
    public double getUnitScale() {
        return 5.0;
    }

    @Override
    public void initUnits(double controlCount) {
        try {
            getView().createControls(getActivity(), (int) Math.round(controlCount));
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }
}
