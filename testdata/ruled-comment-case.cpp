double sample(double demand) {
  // ---- elasticity probe: the first sample past the last act's cooldown is
  // ---- rendered at that act's scalars; its demand against the pre-act
  // ---- demand measures the elasticity of the rung the act moved -------
  if (probe_.armed && cooldownFrames_ == 0) {
    probe_.armed = false;
    if (lodBias_ == probe_.actLod && resScale_ == probe_.actRes) {
      const double lr = std::log(lodBias_ / probe_.lod);
      const double rr = std::log(resScale_ / probe_.res);
      const double dr = std::log(demand / probe_.demand);
      if (rr == 0.0 && std::abs(lr) >= minStep) {
        eLod_ = std::clamp(dr / lr, 0.0, kGovDemandElasticityPrior);
      } else if (lr == 0.0 && std::abs(rr) >= minStep) {
        eRes_ = std::clamp(dr / rr, 0.0, kGovDemandElasticityPrior);
      }
    }
  }
  return demand;
}
