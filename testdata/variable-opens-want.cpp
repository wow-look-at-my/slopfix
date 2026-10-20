void applyQuality(double q);

double run(double lodNow, double resNow, double eLod, double eRes, bool lodOn, bool resPinned) {
  // Pair demand at quality q relative to now. This is q split over the
  // free rungs the way applyQuality spends it (before the res lattice),
  // each rung's ratio raised to its measured elasticity. Non-decreasing
  // in q.
  const auto demandRatio = [&](double q) -> double {
    double lod = lodNow;
    double res = resNow;
    if (lodOn && !resPinned) {
      lod = std::clamp(q, kGovLodBiasMin, 1.0);
      res = std::clamp(q / lod, kGovResScaleMin, 1.0);
    } else if (lodOn) {
      lod = std::clamp(q / std::max(resNow, 0.01), kGovLodBiasMin, 1.0);
    } else if (!resPinned) {
      res = std::clamp(q / lodNow, kGovResScaleMin, 1.0);
    }
    return std::pow(lod / lodNow, eLod) * std::pow(res / resNow, eRes);
  };
  return demandRatio(1.0);
}
