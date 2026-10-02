package calib

import (
	_ "embed"
	"encoding/json"
	"math"
	"sort"
)

// Empirical calibration (SYSTEM_DESIGN.md §8, split conformal): the
// measurement model only knows the noise of each face fit, but lengths and
// areas also move with where corners land, which differs between captures.
// With no laser ground truth, two independent captures of the same element
// are the reference: their difference has variance σa² + σb² + 2τ², where
// τ is the missing per-measurement error. τ is the conformal 90% quantile
// over the benchmark pairs, and is added in quadrature to every interval.
type Empirical struct {
	Tier        string  `json:"tier"`
	LengthTauM  float64 `json:"length_tau_m"` // added to every wall length / opening offset, metres
	AreaTauRel  float64 `json:"area_tau_rel"` // added to every area, relative
	LengthPairs int     `json:"length_pairs"` // benchmark pairs used
	AreaPairs   int     `json:"area_pairs"`
	LengthLOO   float64 `json:"length_loo_cov"` // leave-one-out coverage of the 90% intervals
	AreaLOO     float64 `json:"area_loo_cov"`
	Source      string  `json:"source"`
}

//go:embed empirical_lidar.json
var lidarJSON []byte

// ForTier returns the committed calibration for a tier. Video and photo
// tiers have no repeat captures of their own yet, so they use the LiDAR
// terms (geometry errors are at least as large) on top of their scale
// uncertainty; the report states that they are not separately calibrated.
func ForTier(tier string) Empirical {
	var e Empirical
	if len(lidarJSON) > 0 {
		_ = json.Unmarshal(lidarJSON, &e)
	}
	return e
}

// Pair is one benchmark comparison: the difference between two
// independent estimates and their model sigmas.
type Pair struct {
	Diff, SigA, SigB float64
}

// Tau returns the extra per-measurement sigma that makes 90% of pairs fall
// inside their combined interval (the ceil(0.9 n)-th smallest needed tau),
// and the leave-one-out coverage of that rule. The finite-sample conformal
// rank ceil(0.9(n+1)) would, with the 17 pairs the samples provide, always
// be the single worst pair (a 2.3 m topology mismatch, tau ~1 m); the plain
// empirical quantile is used instead and its leave-one-out coverage is
// reported so the loss of the guarantee is visible.
func Tau(pairs []Pair) (tau, loo float64) {
	need := func(p Pair) float64 {
		v := (p.Diff/z90)*(p.Diff/z90) - p.SigA*p.SigA - p.SigB*p.SigB
		return math.Sqrt(math.Max(v, 0) / 2)
	}
	q := func(ts []float64) float64 {
		s := append([]float64{}, ts...)
		sort.Float64s(s)
		k := int(math.Ceil(0.9*float64(len(s)))) - 1
		return s[min(k, len(s)-1)]
	}
	ts := make([]float64, len(pairs))
	for i, p := range pairs {
		ts[i] = need(p)
	}
	if len(ts) == 0 {
		return 0, 0
	}
	tau = q(ts)
	covered := 0
	for i := range pairs {
		rest := append(append([]float64{}, ts[:i]...), ts[i+1:]...)
		if len(rest) > 0 && ts[i] <= q(rest) {
			covered++
		}
	}
	return tau, float64(covered) / float64(len(pairs))
}
