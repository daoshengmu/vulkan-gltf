package uniform

import (
	"unsafe"
	"github.com/xlab/linmath"
)

type vkTriUniform struct {
	mvp      linmath.Mat4x4
}

const vkTriUniformSize = uint32(unsafe.Sizeof(vkTriUniform{}))

func (u *vkTriUniform) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:vkTriUniformSize]
}

var gVertexData = linmath.ArrayFloat32([]float32{

	-1.0, 1.0, -1.0, 	1, 0, 0,	 // 0
	1.0, 1.0, -1.0,   0, 1, 0,   // 1
	-1.0, -1.0, -1.0, 1, 0, 0,   // 2
	1.0, -1.0, -1.0,  0, 0, 1,   // 3


	-1.0, 1.0, 1.0, 	1, 0, 0,   // 4
	1.0, 1.0, 1.0,    0, 1, 0,   // 5
	-1.0, -1.0, 1.0,  0, 0, 1,   // 6
	1.0, -1.0, 1.0,   0, 1, 0,   // 7
})

var gIndexData = linmath.ArrayUint16([]uint16{
	// -X side
	0, 2, 4, 4, 2, 6,
	// -Z side
	2, 0, 3, 3, 0, 1,
	// -Y side
	2, 7, 6, 2, 3, 7,
	// +Y side
	0, 4, 5, 1, 0, 5,
	// +X side
	7, 3, 1, 7, 1, 5,
	// +Z side
	4, 6, 7, 5, 4, 7,
})
