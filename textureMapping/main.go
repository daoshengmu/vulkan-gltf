package main

import (
	"log"
	"runtime"
	"time"

	"github.com/vulkan-gltf/textureMapping/texture"

	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
	"github.com/vulkan-gltf/util"
)

var appInfo = &vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanUniform\x00",
	PEngineName:        "vulkangltf.com\x00",
}

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

func main() {
	procAddr := glfw.GetVulkanGetInstanceProcAddress()
	if procAddr == nil {
		panic("GetInstanceProcAddress is nil")
	}
	vk.SetGetInstanceProcAddr(procAddr)

	util.OrPanic(glfw.Init())
	util.OrPanic(vk.Init())
	defer closer.Close()

	var (
		r   texture.VulkanRenderInfo
	)

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	const width = 640
	const height = 480

	window, err := glfw.CreateWindow(width, height, "Vulkan uniform buffer", nil, nil)
	util.OrPanic(err)

	createSurface := func(instance interface{}) uintptr {
		surface, err := window.CreateWindowSurface(instance, nil)
		util.OrPanic(err)
		return surface
	}

	r, err = texture.Initialize(appInfo, window.GLFWWindow(), window.GetRequiredInstanceExtensions(),
														  createSurface, float32(width)/float32(height))
	util.OrPanic(err)

	// Some sync logic
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)
	spinAngle := float32(1.0)

	for {
		select {
		case <-exitC:
			texture.DestroyInOrder(&r)
			window.Destroy()
			glfw.Terminate()
			fpsTicker.Stop()
			doneC <- struct{}{}
			return
		case <-fpsTicker.C:
			if window.ShouldClose() {
				exitC <- struct{}{}
				continue
			}
			glfw.PollEvents()
			texture.VulkanDrawFrame(r, spinAngle)
			spinAngle += 1.0
		}
	}
}