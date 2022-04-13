Bitstream file name: CONFCROP_B_VIDYO_2.hevc

Bitstream feature name: Conformance cropping window in VPS

Bitstream feature description:
The conformance cropping window parameters in the VPS (starting with conformance_window_vps_flag) are used to crop the output picture
- Conformance crop offsets
    Layer 0 -   Top: 50 (signalled as 25)
                Bottom: 6 (signalled as 3)
                Left: 160 (signalled as 80)
                Right: 80 (signalled as 40)
    Layer 1 -   Top: 24 (signalled as 12)
                Bottom: 148 (signalled as 74)
                Left: 320 (signalled as 160)
                Right: 0
- Coding structure: Low Delay P
- Bitstream generated using - ConformanceMode = 3: conformance cropping
- Number of Frames - 8

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 720x480
                       Coded:  960x536
- Layer 1 resolution - Output: 1600x900
                       Coded:  1920x1072

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1527

Contact: Won Kap Jang, Vidyo Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

