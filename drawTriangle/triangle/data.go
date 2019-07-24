package triangle

import (
	"unsafe"

	lin "github.com/xlab/linmath"
)

const triCount = 2;

type vkTriUniform struct {
	mvp      lin.Mat4x4
	position [triCount * 3][4]float32
	attr     [triCount * 3][4]float32  // color
}

// const vkTexCubeUniformSize = int(unsafe.Sizeof(vkTexCubeUniform{}))
const vkTriUniformSize = int(unsafe.Sizeof(vkTriUniform{}))

func (u *vkTriUniform) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:vkTriUniformSize]
}

var gVertexBufferData = []float32{
	// -1.0, -1.0, -1.0, // -X side
	// -1.0, -1.0, 1.0,
	// -1.0, 1.0, 1.0,
	// -1.0, 1.0, 1.0,
	// -1.0, 1.0, -1.0,
	// -1.0, -1.0, -1.0,

	// -1.0, -1.0, -1.0, // -Z side
	// 1.0, 1.0, -1.0,
	// 1.0, -1.0, -1.0,
	// -1.0, -1.0, -1.0,
	// -1.0, 1.0, -1.0,
	// 1.0, 1.0, -1.0,

	// -1.0, -1.0, -1.0, // -Y side
	// 1.0, -1.0, -1.0,
	// 1.0, -1.0, 1.0,
	// -1.0, -1.0, -1.0,
	// 1.0, -1.0, 1.0,
	// -1.0, -1.0, 1.0,

	// -1.0, 1.0, -1.0, // +Y side
	// -1.0, 1.0, 1.0,
	// 1.0, 1.0, 1.0,
	// -1.0, 1.0, -1.0,
	// 1.0, 1.0, 1.0,
	// 1.0, 1.0, -1.0,

	// 1.0, 1.0, -1.0, // +X side
	// 1.0, 1.0, 1.0,
	// 1.0, -1.0, 1.0,
	// 1.0, -1.0, 1.0,
	// 1.0, -1.0, -1.0,
	// 1.0, 1.0, -1.0,

	-1.0, 1.0, 1.0, // +Z side
	-1.0, -1.0, 1.0,
	1.0, 1.0, 1.0,
	-1.0, -1.0, 1.0,
	1.0, -1.0, 1.0,
	1.0, 1.0, 1.0,
}

var gColorBufferData = []float32{
	1.0, 0.0, 0.0, 1.0,
	0.0, 1.0, 0.0, 1.0,
	0.0, 0.0, 1.0, 1.0,
	1.0, 0.0, 0.0, 1.0,
	0.0, 1.0, 0.0, 1.0,
	0.0, 0.0, 1.0, 1.0,
}