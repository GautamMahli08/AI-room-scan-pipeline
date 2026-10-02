package pointcloud

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"roomscan/internal/geom"
)

// WritePLY writes points as a binary little-endian PLY (float32 xyz), for
// inspection in MeshLab / CloudCompare.
func WritePLY(path string, pts []geom.Vec3) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "ply\nformat binary_little_endian 1.0\nelement vertex %d\nproperty float x\nproperty float y\nproperty float z\nend_header\n", len(pts))
	var b [12]byte
	for _, p := range pts {
		for i := 0; i < 3; i++ {
			binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(float32(p[i])))
		}
		w.Write(b[:])
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
