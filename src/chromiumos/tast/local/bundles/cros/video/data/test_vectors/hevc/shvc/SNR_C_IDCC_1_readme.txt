Bitstream file name: SNR_C_IDCC_1.hevc

Bitstream feature name: 3 SNR layers

Bitstream feature description:
SNR scalability coded video at a single spatial resolution but at different qualities, the lower layer can be used to predict the high layer to reduce the bits.

Number of Layers: 3
- Layer 0 	: 	QP0 = 22
- Layer 1 	: 	QP1 = 20
- Layer 1	: 	NumSamplePredRefLayers1 = 1
- Layer 1	: 	SamplePredRefLayerIds1 = 0
- Layer 1	: 	NumMotionPredRefLayers1 = 1
- Layer 1	: 	MotionPredRefLayerIds1 = 0
- Layer 1	: 	NumActiveRefLayers1 = 1
- Layer 1	: 	PredLayerIds1 = 0
- Layer 2 	: 	QP1 = 16
- Layer 2	: 	NumSamplePredRefLayers2 = 1
- Layer 2	: 	SamplePredRefLayerIds2 = 1
- Layer 2	: 	NumMotionPredRefLayers2 = 1
- Layer 2	: 	MotionPredRefLayerIds2 = 1
- Layer 2	: 	NumActiveRefLayers2 = 1
- Layer 2	: 	PredLayerIds2 = 0

- Coding structure: Low delay B				
- Number of Frames - 4

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 4)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 4)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 4)
            Layer: 1, PTL Idx: 2 (Scalable Main 4)
            Layer: 2, PTL Idx: 2 (Scalable Main 4)

DefaultTargetOutputLayerIdc   : 1
			
Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 4
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 4
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 1920x1080
                       Coded:  1920x1080
- Layer 1 resolution - Output: 1920x1080
                       Coded:  1920x1280
- Layer 2 resolution - Output: 1920x1080
                       Coded:  1920x1280
Frame rate: 24 fps for both layers

SHM Version: SHM Dev branch, rev 1021

Contact: Yong He, InterDigital Communications, LLC (Yong.He@InterDigital.com)
		 Yan Ye,  InterDigital Communications, LLC (Yan.Ye@InterDigital.com)
