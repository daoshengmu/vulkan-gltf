package main

import (
	"log"
	"runtime"
	"time"

	"github.com/vulkan-gltf/drawTriangle/triangle"

	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

var appInfo = &vk.ApplicationInfo{
	SType:              vk.StructureTypeApplicationInfo,
	ApiVersion:         vk.MakeVersion(1, 0, 0),
	ApplicationVersion: vk.MakeVersion(1, 0, 0),
	PApplicationName:   "VulkanTriangle\x00",
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
		v   triangle.VulkanDeviceInfo
		s   triangle.VulkanSwapchainInfo
		r   triangle.VulkanRenderInfo
		vb   triangle.VulkanBufferInfo
		ib   triangle.VulkanBufferInfo
		gfx triangle.VulkanGfxPipelineInfo
	)

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	window, err := glfw.CreateWindow(640, 480, "Vulkan Info", nil, nil)
	orPanic(err)

	createSurface := func(instance interface{}) uintptr {
		surface, err := window.CreateWindowSurface(instance, nil)
		orPanic(err)
		return surface
	}

	v, err = triangle.NewVulkanDevice(appInfo,
		window.GLFWWindow(),
		window.GetRequiredInstanceExtensions(),
		createSurface)
	orPanic(err)

	s, err = v.CreateSwapchain()
	orPanic(err)
	r, err = triangle.CreateRenderer(v.Device, s.DisplayFormat)
	orPanic(err)
	err = s.CreateFramebuffers(r.RenderPass, nil)
	orPanic(err)
	vb, err = v.CreateVertexBuffers()
	orPanic(err)
	ib, err = v.CreateIndexBuffers()
	orPanic(err)
	gfx, err = triangle.CreateGraphicsPipeline(v.Device, s.DisplaySize, r.RenderPass)
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
	triangle.VulkanInit(&v, &s, &r, &vb, &ib, &gfx)

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)
	for {
		select {
		case <-exitC:
			triangle.DestroyInOrder(&v, &s, &r, &vb, &ib, &gfx)
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
			triangle.VulkanDrawFrame(v, s, r)
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