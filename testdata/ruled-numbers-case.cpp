void testInertLodRung() {
  // ---- (l) INERT LOD RUNG: the M5 animation telemetry (1.23M splats per
  // ---- frame, 1280x720 internal, fgen feed budget 1000/36 - 4 = 23.8 ms).
  // ---- Measured headless: lod-bias 1 -> 0.42 moved pair demand by -0.2%,
  // ---- and res 1 -> 0.5 moved it 3.41M -> 2.60M (elasticity 0.39). Raster
  // ---- 50.0 ms at res 1 and 19.4 ms at res 0.5 give kPx 39.8 ms/Mpx and
  // ---- kPairR 3.94 ns/pair; bin+scatter+sort 3.8 ms gives kBss 1.12. Only
  // ---- the res floor lands inside the deadband. The governor must learn
  // ---- both elasticities from its own acts and reach that floor without
  // ---- walking the inert lod rung act by act.
  {
    Governor gov;
    Governor::Config cfg;
    cfg.budgetMs = 1000.0 / 36.0 - 4.0;
    cfg.lodAvailable = true;
    World w;
    w.demand0 = 3.41e6;
    w.kPx = 39.8;
    w.kPairR = 3.94;
    w.kBss = 1.12;
    w.fixed = 1.0;
    w.canvasW = 1280;
    w.canvasH = 720;
    w.eLod = 0.0;
    w.eRes = 0.39;
    int floorSample = -1;
    int lodOnlyActs = 0;
    int acts = 0;
    for (int i = 0; i < 200; i++) {
      Governor::Sample s = w.step(gov);
      const double lodBefore = gov.lodBias();
      const double resBefore = gov.resolutionScale();
      if (gov.sample(cfg, s)) {
        acts++;
        if (gov.resolutionScale() == resBefore && gov.lodBias() != lodBefore) lodOnlyActs++;
      }
      if (floorSample < 0 && gov.resolutionScale() == kGovResScaleMin) floorSample = i;
    }
  }
}
