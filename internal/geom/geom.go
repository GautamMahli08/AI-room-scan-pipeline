// Package geom holds the small fixed-size linear algebra used throughout the
// pipeline. Larger problems (SVD, eigen decomposition, least squares) use gonum.
package geom

import "math"

// Vec3 is a 3D vector in metres unless stated otherwise.
type Vec3 [3]float64

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a[0] * s, a[1] * s, a[2] * s} }
func (a Vec3) Dot(b Vec3) float64   { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func (a Vec3) Norm() float64        { return math.Sqrt(a.Dot(a)) }

func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// Unit returns a/|a|, or the zero vector if a is zero.
func (a Vec3) Unit() Vec3 {
	n := a.Norm()
	if n == 0 {
		return Vec3{}
	}
	return a.Scale(1 / n)
}

// Mat3 is a row-major 3x3 matrix.
type Mat3 [3][3]float64

func Identity3() Mat3 { return Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} }

func (m Mat3) MulVec(v Vec3) Vec3 {
	return Vec3{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func (m Mat3) Mul(n Mat3) Mat3 {
	var r Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i][j] = m[i][0]*n[0][j] + m[i][1]*n[1][j] + m[i][2]*n[2][j]
		}
	}
	return r
}

func (m Mat3) T() Mat3 {
	return Mat3{
		{m[0][0], m[1][0], m[2][0]},
		{m[0][1], m[1][1], m[2][1]},
		{m[0][2], m[1][2], m[2][2]},
	}
}

// Quat is a rotation quaternion stored as (x, y, z, w).
type Quat struct{ X, Y, Z, W float64 }

// Mat returns the rotation matrix of q. q is normalised first, so slightly
// non-unit quaternions from CSV round-off are tolerated.
func (q Quat) Mat() Mat3 {
	n := math.Sqrt(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W)
	x, y, z, w := q.X/n, q.Y/n, q.Z/n, q.W/n
	return Mat3{
		{1 - 2*(y*y+z*z), 2 * (x*y - z*w), 2 * (x*z + y*w)},
		{2 * (x*y + z*w), 1 - 2*(x*x+z*z), 2 * (y*z - x*w)},
		{2 * (x*z - y*w), 2 * (y*z + x*w), 1 - 2*(x*x+y*y)},
	}
}

// RotY returns a rotation of theta radians about the +Y axis.
func RotY(theta float64) Mat3 {
	c, s := math.Cos(theta), math.Sin(theta)
	return Mat3{{c, 0, s}, {0, 1, 0}, {-s, 0, c}}
}
