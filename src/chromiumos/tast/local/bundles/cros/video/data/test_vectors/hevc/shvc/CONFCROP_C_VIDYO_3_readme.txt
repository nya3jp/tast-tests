Bitstream file name: CONFCROP_C_VIDYO_3.hevc

Bitstream feature name: Conformance cropping window in VPS

Bitstream feature description:
The conformance cropping window parameters in the VPS (starting with conformance_window_vps_flag) are used to crop the output picture
- Conformance crop offsets
    Layer 0 -   Top: 0
                Bottom: 4 (signalled as 2)
                Left: 0
                Right: 0
    Layer 1 -   Top: 0
                Bottom: 180 (signalled as 90)
                Left: 0
                Right: 320 (signalled as 160)
    Layer 2 -   Top: 80 (signalled as 40)
                Bottom: 100 (signalled as 50)
                Left: 120 (signalled as 60)
                Right: 40 (signalled as 20)
- Coding structure: Low Delay P
- Bitstream generated using - Layer 0: ConformanceMode = 1: automatic padding
                              Layer 1: ConformanceMode = 2: user specified padding
                              Layer 2: ConformanceMode = 3: conformance cropping
- Number of Frames - 4

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)
            Layer: 2, PTL Idx: 3 (Scalable Main 4)

Number of layers: 3

Profile, Tier, Level information: Num PTL = 4
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3.1
- PTL_Idx 3 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 960x540
                       Coded:  960x544
- Layer 1 resolution - Output: 960X540
                       Coded:  1280X720
- Layer 2 resolution - Output: 1760x900
                       Coded:  1920x1080

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1527

Contact: Won Kap Jung, Vidyo Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

