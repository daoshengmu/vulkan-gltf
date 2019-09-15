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
		v   uniform.VulkanDeviceInfo
		s   uniform.VulkanSwapchainInfo
		r   uniform.VulkanRenderInfo
		vb  uniform.VulkanBufferInfo
		ib  uniform.VulkanBufferInfo
		// ub  uniform.Buffer
	//	um	vk.DeviceMemory
		gfx uniform.VulkanGfxPipelineInfo
	)

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	window, err := glfw.CreateWindow(640, 480, "Vulkan Info", nil, nil)
	orPanic(err)

	createSurface := func(instance interface{}) uintptr {
		surface, err := window.CreateWindowSurface(instance, nil)
		orPanic(err)
		return surface
	}

	v, err = uniform.NewVulkanDevice(appInfo,
		window.GLFWWindow(),
		window.GetRequiredInstanceExtensions(),
		createSurface)
	orPanic(err)

	s, err = v.CreateSwapchain()
	orPanic(err)
	r, err = uniform.CreateRenderer(v.Device, s.DisplayFormat)
	orPanic(err)
	err = s.CreateDescriptorPool()
	orPanic(err)
	err = s.CreateDescriptorSet()
	orPanic(err)
	err = s.CreateFramebuffers(r.RenderPass, nil)
	orPanic(err)
	vb, err = v.CreateVertexBuffers()
	orPanic(err)
	ib, err = v.CreateIndexBuffers()
	orPanic(err)
	// create uniform buffer and MVP matrix.
	//var buf *uniform.Buffer
	// buf, err = v.CreateUniformBuffers()
	// ub = *buf;
	// orPanic(err)

	gfx, err = uniform.CreateGraphicsPipeline(v.Device, s.DisplaySize, r.RenderPass, s.DescLayout)
	orPanic(err)
	log.Println("[INFO] swapchain lengths:", s.SwapchainLen)
	err = r.CreateCommandBuffers(s.DefaultSwapchainLen())
	orPanic(err)

	// Some sync logic
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})
	uniform.VulkanInit(&v, &s, &r, &vb, &ib, &gfx)

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)
	for {
		select {
		case <-exitC:
			uniform.DestroyInOrder(&v, &s, &r, &vb, &ib, &gfx)
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

			// rotate cube
			// set unifrom

			uniform.VulkanDrawFrame(v, s, r)
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