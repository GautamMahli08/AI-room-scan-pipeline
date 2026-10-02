// Package calib turns measurement evidence into 90% intervals.
//
// Two layers (SYSTEM_DESIGN.md §8): a measurement model that propagates
// fit residuals, support and known sensor error to each dimension (this
// file), and an empirical split-conformal widening per tier applied on top
// once benchmark residuals exist.
package calib

import "math"

// z90 is the two-sided 90% normal quantile.
const z90 = 1.6449

// Sensor error floors for the LiDAR tier, metres. ARKit LiDAR depth has a
// systematic per-surface bias of roughly half a centimetre at room ranges
// that does not average away with more points.
const (
	LiDARFaceSys    = 0.005 // wall face position
	LiDARHeightSys  = 0.005 // floor/ceiling plane position
	InferredFaceSig = 0.10  // a wall placed from free space only, not observed
)

// Interval is a symmetric 90% interval around v with standard deviation sigma.
func Interval(v, sigma float64) [2]float64 {
	return [2]float64{v - z90*sigma, v + z90*sigma}
}

// FaceSigma is the standard deviation of a fitted plane position from n
// points with the given spread, plus the systematic floor.
func FaceSigma(spread float64, n int, sys float64) float64 {
	if n <= 0 {
		return InferredFaceSig
	}
	return math.Sqrt(spread*spread/float64(n) + sys*sys)
}

// LengthSigma is the uncertainty of a wall length bounded by two faces.
func LengthSigma(endA, endB float64) float64 { return math.Hypot(endA, endB) }

// HeightSigma is the uncertainty of a ceiling height: both the ceiling and
// the floor plane contribute.
func HeightSigma(ceil, floor float64) float64 { return math.Hypot(ceil, floor) }
