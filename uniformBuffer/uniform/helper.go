package uniform

import (
	"log"
	"unsafe"
	vk "github.com/vulkan-go/vulkan"
)

func orPanic(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}
		panic(err)
	}
}

func check(ret vk.Result, name string) bool {
	if err := vk.Error(ret); err != nil {
		log.Println("[WARN]", name, "failed with", err)
		return true
	}
	return false
}

func repackUint32(data []byte) []uint32 {
	buf := make([]uint32, len(data)/4)
	vk.Memcopy(unsafe.Pointer((*sliceHeader)(unsafe.Pointer(&buf)).Data), data)
	return buf
}

type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}