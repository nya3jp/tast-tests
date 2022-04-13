Bitstream file name: MAXTID_B_ETRI_1.bit

Bitstream feature name: max_tid_il_ref_pics_plus1 in VPS extension

Bitstream feature description:
The max_tid_il_ref_pics_plus1 in VPS extension are used to support controlling the use of inter-layer prediction based on temporal sub-layer

- max_tid_ref_presenet_flag = 0

- Coding structure: Random Acess with 4 temporal sub-layers
- Number of Frames: 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3.1)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 3.1)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
            Layer: 2, PTL Idx: 2 (Scalable Main 4)

Number of layers: 3

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3.1
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 960x540
                       Coded:  960x544
- Layer 1 resolution - Coded:  1920x1080
- Layer 2 resolution - Coded:  1920x1080

Frame rate: 24 fps for all layers

SHM Version: SHM Dev branch, rev 1025

Contact: Hahyun Lee, ETRI (hanilee@etri.re.kr)
         Jung Won Kang, ETRI (jungwon@etri.re.kr)
