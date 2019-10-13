package main

import (
	"log"
	"runtime"
	"time"

	"github.com/vulkan-gltf/uniformBuffer/uniform"

	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
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

	orPanic(glfw.Init())
	orPanic(vk.Init())
	defer closer.Close()

	var (
		// v   renderer.VulkanDeviceInfo
		// s   renderer.VulkanSwapchainInfo
		r   uniform.VulkanRenderInfo
		// vb  renderer.VulkanBufferInfo
		// ib  renderer.VulkanBufferInfo
	//	gfx uniform.VulkanGfxPipelineInfo
	)

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	const width = 640
	const height = 480

	window, err := glfw.CreateWindow(width, height, "Vulkan uniform buffer", nil, nil)
	orPanic(err)

	createSurface := func(instance interface{}) uintptr {
		surface, err := window.CreateWindowSurface(instance, nil)
		orPanic(err)
		return surface
	}

	// v, err = renderer.NewVulkanDevice(appInfo,
	// 	window.GLFWWindow(),
	// 	window.GetRequiredInstanceExtensions(),
	// 	createSurface)
	// orPanic(err)

	// s, err = v.CreateSwapchain()
	// orPanic(err)
	// r, err = uniform.CreateRenderer(v.Device, s.DisplayFormat, float32(width)/float32(height))
	// orPanic(err)
	r, err = uniform.Initialize(appInfo, window.GLFWWindow(), window.GetRequiredInstanceExtensions(),
														  createSurface, float32(width)/float32(height))
	orPanic(err)
//	err = s.CreateDescriptorPool()
//	orPanic(err)
//	err = s.CreateDescriptorSet(vk.DeviceSize(uniform.UniformDataSize()))
	//orPanic(err)
//	err = s.CreateFramebuffers(r.RenderPass, nil)
//	orPanic(err)
	// vb, err = v.CreateVertexBuffers()
	// orPanic(err)
	// ib, err = v.CreateIndexBuffers()
	// orPanic(err)

	// TODO: move to uniform
	// gfx, err = uniform.CreateGraphicsPipeline(v.Device, s.DisplaySize, r.RenderPass, s.DescLayout)
	// orPanic(err)
	// log.Println("[INFO] swapchain lengths:", s.SwapchainLen)
	// err = r.CreateCommandBuffers(s.DefaultSwapchainLen())
	// orPanic(err)

	// Some sync logic
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})
	// uniform.VulkanInit(&v, &s, &r, &vb, &ib, &gfx)

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)
	spinAngle := float32(1.0)

	for {
		select {
		case <-exitC:
			// uniform.DestroyInOrder(&v, &s, &r, &vb, &ib, &gfx)
			uniform.DestroyInOrder(&r)
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
			uniform.VulkanDrawFrame(r, spinAngle)
			spinAngle += 1.0
		}
	}
}

func orPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(err)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	}
}