Name: 3DHC_DT_H
Specification: The bitstream is coded with AI configuration as well as 3-view configuration. In addition, the all depth Intra tools are enabled together with DLT
Functional stage: Decoding of three views with depth Intra coding tools enabled.
Purpose: To conform the depth Intra coding tools in AI configuration.
frame number:64
Software: HTM15.1
Configure(base): baseCfg_3view+depth_AllIntra.cfg + qpCfg_Nview+depth_QP40.cfg + seqCfg_Kendo.cfg

Configure: --IvMvPredFlag 0 0 --IvResPredFlag 0 --IlluCompEnable 0 --IlluCompLowLatencyEnc 0 --ViewSynthesisPredFlag 0 --DepthRefinementFlag 0 --IvMvScalingFlag 0 --Log2SubPbSizeMinus3 3 --Log2MpiSubPbSizeMinus3 3 --DepthBasedBlkPartFlag 0 --VSO 1 --IntraWedgeFlag 1 --IntraContourFlag 0 --IntraSdcFlag 1 --DLT 1 --QTL 0 --QtPredFlag 0 --InterSdcFlag 0 --MpiFlag 0 --DepthIntraSkip 1