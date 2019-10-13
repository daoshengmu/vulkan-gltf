package uniform

import (
	"fmt"
	"log"
	"unsafe"

	"github.com/xlab/linmath"
	vk "github.com/vulkan-go/vulkan"
	"github.com/vulkan-gltf/renderer"
	"github.com/vulkan-gltf/util"
)

// // enableDebug is disabled by default since VK_EXT_debug_report
// // is not guaranteed to be present on a device.
// // Nvidia Shield K1 fw 1.3.0 lacks this extension,
// // on fw 1.2.0 it works fine.
// const enableDebug = false

type VulkanRenderInfo struct {
	device vk.Device

	RenderPass vk.RenderPass
	cmdPool    vk.CommandPool
	cmdBuffers []vk.CommandBuffer
	semaphores []vk.Semaphore
	fences     []vk.Fence

	viewMatrix	linmath.Mat4x4
	projectionMatrix linmath.Mat4x4
}

type VulkanGfxPipelineInfo struct {
	device vk.Device

	pipelineLayout   vk.PipelineLayout
	pipelineCache    vk.PipelineCache
	pipeline 				 vk.Pipeline
}

func (v *VulkanRenderInfo) DefaultFence() vk.Fence {
	return v.fences[0]
}

func (v *VulkanRenderInfo) DefaultSemaphore() vk.Semaphore {
	return v.semaphores[0]
}

func vulkanInit() {

	clearValues := []vk.ClearValue{
		vk.NewClearValue([]float32{0.0, 0.0, 0.0, 1}),
	}
	for i := range r.cmdBuffers {
		cmdBufferBeginInfo := vk.CommandBufferBeginInfo{
			SType: vk.StructureTypeCommandBufferBeginInfo,
		}
		renderPassBeginInfo := vk.RenderPassBeginInfo{
			SType:       vk.StructureTypeRenderPassBeginInfo,
			RenderPass:  r.RenderPass,
			Framebuffer: s.Framebuffers[i],
			RenderArea: vk.Rect2D{
				Offset: vk.Offset2D{
					X: 0, Y: 0,
				},
				Extent: s.DisplaySize,
			},
			ClearValueCount: 1,
			PClearValues:    clearValues,
		}
		ret := vk.BeginCommandBuffer(r.cmdBuffers[i], &cmdBufferBeginInfo)
		util.Check(ret, "vk.BeginCommandBuffer")

		vk.CmdBeginRenderPass(r.cmdBuffers[i], &renderPassBeginInfo, vk.SubpassContentsInline)
		vk.CmdBindPipeline(r.cmdBuffers[i], vk.PipelineBindPointGraphics, gfx.pipeline)
		offsets := make([]vk.DeviceSize, vb.GetBufferLen())
		vk.CmdBindDescriptorSets(r.cmdBuffers[i], vk.PipelineBindPointGraphics, gfx.pipelineLayout,
			0, 1, []vk.DescriptorSet{s.DescriptorSet[i]}, 0, nil)

		vk.CmdBindVertexBuffers(r.cmdBuffers[i], 0, 1, *vb.GetBuffers(), offsets)
		vk.CmdBindIndexBuffer(r.cmdBuffers[i], ib.DefaultBuffer(), 0, vk.IndexTypeUint16);
		vk.CmdDrawIndexed(r.cmdBuffers[i], (uint32)(len(gIndexData)), 1, 0, 0, 0)
		vk.CmdEndRenderPass(r.cmdBuffers[i])

		ret = vk.EndCommandBuffer(r.cmdBuffers[i])
		util.Check(ret, "vk.EndCommandBuffer")
	}
	fenceCreateInfo := vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
	}
	semaphoreCreateInfo := vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	r.fences = make([]vk.Fence, 1)
	ret := vk.CreateFence(v.Device, &fenceCreateInfo, nil, &r.fences[0])
	util.Check(ret, "vk.CreateFence")
	r.semaphores = make([]vk.Semaphore, 1)
	ret = vk.CreateSemaphore(v.Device, &semaphoreCreateInfo, nil, &r.semaphores[0])
	util.Check(ret, "vk.CreateSemaphore")
}

func LoadShader(device vk.Device, name string) (vk.ShaderModule, error) {
	var module vk.ShaderModule
	data, err := Asset(name)
	if err != nil {
		err := fmt.Errorf("asset %s not found: %s", name, err)
		return module, err
	}

	// Phase 1: vk.CreateShaderModule

	shaderModuleCreateInfo := vk.ShaderModuleCreateInfo{
		SType:    vk.StructureTypeShaderModuleCreateInfo,
		CodeSize: uint(len(data)),
		PCode:    repackUint32(data),
	}
	err = vk.Error(vk.CreateShaderModule(device, &shaderModuleCreateInfo, nil, &module))
	if err != nil {
		err = fmt.Errorf("vk.CreateShaderModule failed with %s", err)
		return module, err
	}
	return module, nil
}

func createGraphicsPipeline(device vk.Device,
	displaySize vk.Extent2D, renderPass vk.RenderPass, descLayout vk.DescriptorSetLayout) (VulkanGfxPipelineInfo, error) {

	var gfxPipeline VulkanGfxPipelineInfo
	// Phase 1: vk.CreatePipelineLayout
	//			create pipeline layout (empty)

	pipelineLayoutCreateInfo := vk.PipelineLayoutCreateInfo{
		SType: vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: 1,
		PSetLayouts: []vk.DescriptorSetLayout{
			descLayout,
		},
	}
	err := vk.Error(vk.CreatePipelineLayout(device, &pipelineLayoutCreateInfo, nil, &gfxPipeline.pipelineLayout))
	if err != nil {
		err = fmt.Errorf("vk.CreatePipelineLayout failed with %s", err)
		return gfxPipeline, err
	}
	dynamicState := vk.PipelineDynamicStateCreateInfo{
		SType: vk.StructureTypePipelineDynamicStateCreateInfo,
		// no dynamic state for this demo
	}

	// Phase 2: load shaders and specify shader stages

	vertexShader, err := LoadShader(device, "shaders/tri-vert.spv")
	if err != nil { // err has enough info
		return gfxPipeline, err
	}
	defer vk.DestroyShaderModule(device, vertexShader, nil)

	fragmentShader, err := LoadShader(device, "shaders/tri-frag.spv")
	if err != nil { // err has enough info
		return gfxPipeline, err
	}
	defer vk.DestroyShaderModule(device, fragmentShader, nil)

	shaderStages := []vk.PipelineShaderStageCreateInfo{
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vertexShader,
			PName:  "main\x00",
		},
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fragmentShader,
			PName:  "main\x00",
		},
	}

	// Phase 3: specify viewport state

	viewports := []vk.Viewport{{
		MinDepth: 0.0,
		MaxDepth: 1.0,
		X:        0,
		Y:        0,
		Width:    float32(displaySize.Width),
		Height:   float32(displaySize.Height),
	}}
	scissors := []vk.Rect2D{{
		Extent: displaySize,
		Offset: vk.Offset2D{
			X: 0, Y: 0,
		},
	}}
	viewportState := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		PViewports:    viewports,
		ScissorCount:  1,
		PScissors:     scissors,
	}

	// Phase 4: specify multisample state
	//					color blend state
	//					rasterizer state

	sampleMask := []vk.SampleMask{vk.SampleMask(vk.MaxUint32)}
	multisampleState := vk.PipelineMultisampleStateCreateInfo{
		SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
		RasterizationSamples: vk.SampleCount1Bit,
		SampleShadingEnable:  vk.False,
		PSampleMask:          sampleMask,
	}
	attachmentStates := []vk.PipelineColorBlendAttachmentState{{
		ColorWriteMask: vk.ColorComponentFlags(
			vk.ColorComponentRBit | vk.ColorComponentGBit |
				vk.ColorComponentBBit | vk.ColorComponentABit,
		),
		BlendEnable: vk.False,
	}}
	colorBlendState := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		LogicOpEnable:   vk.False,
		LogicOp:         vk.LogicOpCopy,
		AttachmentCount: 1,
		PAttachments:    attachmentStates,
	}
	rasterState := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeFill,
		CullMode:                vk.CullModeFlags(vk.CullModeBackBit),
		FrontFace:               vk.FrontFaceCounterClockwise,
		DepthBiasEnable:         vk.False,
		LineWidth:               1,
	}

	// Phase 5: specify input assembly state
	//					vertex input state and attributes

	inputAssemblyState := vk.PipelineInputAssemblyStateCreateInfo{
		SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology:               vk.PrimitiveTopologyTriangleList,
		PrimitiveRestartEnable: vk.True,
	}
	vertexInputBindings := []vk.VertexInputBindingDescription{{
		Binding:   0,
		Stride:    6 * 4, // 4 = sizeof(float32)
		InputRate: vk.VertexInputRateVertex,
	}}
	vertexInputAttributes := []vk.VertexInputAttributeDescription{{
		Binding:  0,
		Location: 0,
		Format:   vk.FormatR32g32b32Sfloat,
		Offset:   0,
	},
	{
		Binding:  0,
		Location: 1,
		Format:   vk.FormatR32g32b32Sfloat,
		Offset:   3 * 4, // 4 = sizeof(float32)
	}}
	vertexInputState := vk.PipelineVertexInputStateCreateInfo{
		SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
		VertexBindingDescriptionCount:   1,
		PVertexBindingDescriptions:      vertexInputBindings,
		VertexAttributeDescriptionCount: uint32(len(vertexInputAttributes)),//1,
		PVertexAttributeDescriptions:    vertexInputAttributes,
	}

	// Phase 5: vk.CreatePipelineCache
	//			vk.CreateGraphicsPipelines

	pipelineCacheInfo := vk.PipelineCacheCreateInfo{
		SType: vk.StructureTypePipelineCacheCreateInfo,
	}
	err = vk.Error(vk.CreatePipelineCache(device, &pipelineCacheInfo, nil, &gfxPipeline.pipelineCache))
	if err != nil {
		err = fmt.Errorf("vk.CreatePipelineCache failed with %s", err)
		return gfxPipeline, err
	}
	pipelineCreateInfos := []vk.GraphicsPipelineCreateInfo{{
		SType:               vk.StructureTypeGraphicsPipelineCreateInfo,
		StageCount:          2, // vert + frag
		PStages:             shaderStages,
		PVertexInputState:   &vertexInputState,
		PInputAssemblyState: &inputAssemblyState,
		PViewportState:      &viewportState,
		PRasterizationState: &rasterState,
		PMultisampleState:   &multisampleState,
		PColorBlendState:    &colorBlendState,
		PDynamicState:       &dynamicState,
		Layout:              gfxPipeline.pipelineLayout,
		RenderPass:          renderPass,
	}}
	pipelines := make([]vk.Pipeline, 1)
	err = vk.Error(vk.CreateGraphicsPipelines(device,
		gfxPipeline.pipelineCache, 1, pipelineCreateInfos, nil, pipelines))
	if err != nil {
		err = fmt.Errorf("vk.CreateGraphicsPipelines failed with %s", err)
		return gfxPipeline, err
	}
	gfxPipeline.pipeline = pipelines[0]
	gfxPipeline.device = device

	return gfxPipeline, nil
}

func VulkanDrawFrame(r VulkanRenderInfo, spinAngle float32) bool {
	var nextIdx uint32

	// Phase 1: vk.AcquireNextImage
	// 			get the framebuffer index we should draw in
	//
	//			N.B. non-infinite timeouts may be not yet implemented
	//			by your Vulkan driver

	err := vk.Error(vk.AcquireNextImage(v.Device, s.DefaultSwapchain(),
		vk.MaxUint64, r.DefaultSemaphore(), vk.NullFence, &nextIdx))
	if err != nil {
		err = fmt.Errorf("vk.AcquireNextImage failed with %s", err)
		log.Println("[WARN]", err)
		return false
	}

	// Rotate cube and set uniform buffer
	var MVP linmath.Mat4x4
	var modelMatrix linmath.Mat4x4
	modelMatrix.Identity()
	modelMatrix.Rotate(&modelMatrix, 0.0, 1.0, 0.0, linmath.DegreesToRadians(spinAngle))
	MVP.Mult(&r.projectionMatrix, &r.viewMatrix)
	MVP.Mult(&MVP, &modelMatrix)
	data := MVP.Data()
	var pData unsafe.Pointer

	vk.MapMemory(v.Device, s.UniformBuffer[nextIdx].GetMemory(), 0, vk.DeviceSize(len(data)), 0, &pData)
	n := vk.Memcopy(pData, data)
	if n != len(data) {
		log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
	}
	vk.UnmapMemory(v.Device, s.UniformBuffer[nextIdx].GetMemory())

	// Phase 2: vk.QueueSubmit
	//			vk.WaitForFences

	vk.ResetFences(v.Device, 1, r.fences)
	submitInfo := []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		WaitSemaphoreCount: 1,
		PWaitSemaphores:    r.semaphores,
		CommandBufferCount: 1,
		PCommandBuffers:    r.cmdBuffers[nextIdx:],
	}}
	err = vk.Error(vk.QueueSubmit(v.Queue, 1, submitInfo, r.DefaultFence()))
	if err != nil {
		err = fmt.Errorf("vk.QueueSubmit failed with %s", err)
		log.Println("[WARN]", err)
		return false
	}

	const timeoutNano = 10 * 1000 * 1000 * 1000 // 10 sec
	err = vk.Error(vk.WaitForFences(v.Device, 1, r.fences, vk.True, timeoutNano))
	if err != nil {
		err = fmt.Errorf("vk.WaitForFences failed with %s", err)
		log.Println("[WARN]", err)
		return false
	}

	// Phase 3: vk.QueuePresent

	imageIndices := []uint32{nextIdx}
	presentInfo := vk.PresentInfo{
		SType:          vk.StructureTypePresentInfo,
		SwapchainCount: 1,
		PSwapchains:    s.Swapchains,
		PImageIndices:  imageIndices,
	}
	err = vk.Error(vk.QueuePresent(v.Queue, &presentInfo))
	if err != nil {
		err = fmt.Errorf("vk.QueuePresent failed with %s", err)
		log.Println("[WARN]", err)
		return false
	}
	return true
}

func (r *VulkanRenderInfo) createCommandBuffers(n uint32) error {
	r.cmdBuffers = make([]vk.CommandBuffer, n)
	cmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        r.cmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: n,
	}
	err := vk.Error(vk.AllocateCommandBuffers(r.device, &cmdBufferAllocateInfo, r.cmdBuffers))
	if err != nil {
		err = fmt.Errorf("vk.AllocateCommandBuffers failed with %s", err)
		return err
	}
	return nil
}

func createRenderer(device vk.Device, displayFormat vk.Format, aspect float32) (VulkanRenderInfo, error) {
	attachmentDescriptions := []vk.AttachmentDescription{{
		Format:         displayFormat,
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpStore,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		InitialLayout:  vk.ImageLayoutColorAttachmentOptimal,
		FinalLayout:    vk.ImageLayoutColorAttachmentOptimal,
	}}
	colorAttachments := []vk.AttachmentReference{{
		Attachment: 0,
		Layout:     vk.ImageLayoutColorAttachmentOptimal,
	}}
	subpassDescriptions := []vk.SubpassDescription{{
		PipelineBindPoint:    vk.PipelineBindPointGraphics,
		ColorAttachmentCount: 1,
		PColorAttachments:    colorAttachments,
	}}
	renderPassCreateInfo := vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: 1,
		PAttachments:    attachmentDescriptions,
		SubpassCount:    1,
		PSubpasses:      subpassDescriptions,
	}
	cmdPoolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateResetCommandBufferBit),
		QueueFamilyIndex: 0,
	}
	var r VulkanRenderInfo
	err := vk.Error(vk.CreateRenderPass(device, &renderPassCreateInfo, nil, &r.RenderPass))
	if err != nil {
		err = fmt.Errorf("vk.CreateRenderPass failed with %s", err)
		return r, err
	}
	err = vk.Error(vk.CreateCommandPool(device, &cmdPoolCreateInfo, nil, &r.cmdPool))
	if err != nil {
		err = fmt.Errorf("vk.CreateCommandPool failed with %s", err)
		return r, err
	}

	// Create MVP matrix
	eyeVec := &linmath.Vec3{0.0, 3.0, 5.0}
	origin := &linmath.Vec3{0.0, 0.0, 0.0}
	upVec := &linmath.Vec3{0.0, 1.0, 0.0}

	r.projectionMatrix.Perspective(linmath.DegreesToRadians(45.0), aspect, 0.1, 100.0);
	r.viewMatrix.LookAt(eyeVec, origin, upVec)
	r.projectionMatrix[1][1] *= -1 // Flip projection matrix from GL to Vulkan orientation.

	r.device = device
	return r, nil
}

var (
	v   renderer.VulkanDeviceInfo
	s   renderer.VulkanSwapchainInfo
	r   VulkanRenderInfo
	vb  renderer.VulkanBufferInfo
	ib  renderer.VulkanBufferInfo
	gfx VulkanGfxPipelineInfo
)

func Initialize(appInfo *vk.ApplicationInfo, window uintptr, instanceExtensions []string,
								createSurfaceFunc func(interface{}) uintptr, ratio float32) (VulkanRenderInfo, error) {

	var err error
	v, err = renderer.NewVulkanDevice(appInfo, window, instanceExtensions, createSurfaceFunc)
	if err != nil {
		err = fmt.Errorf("renderer.NewVulkanDevice failed with %s", err)
		return r, err
	}

	var MVP linmath.Mat4x4
	uniformData := vkTriUniform{
		mvp: MVP,
	}

	s, err = v.CreateSwapchain(uniformData.Data())
	if err != nil {
		err = fmt.Errorf("renderer.CreateSwapchain failed with %s", err)
		return r, err
	}
	r, err = createRenderer(v.Device, s.DisplayFormat, ratio)
	if err != nil {
		err = fmt.Errorf("renderer.createRenderer failed with %s", err)
		return r, err
	}
	err = s.CreateDescriptorPool()
	if err != nil {
		err = fmt.Errorf("renderer.CreateDescriptorPool failed with %s", err)
		return r, err
	}
	err = s.CreateDescriptorSet(vk.DeviceSize(len(uniformData.Data())))
	if err != nil {
		err = fmt.Errorf("renderer.CreateDescriptorSet failed with %s", err)
		return r, err
	}
	err = s.CreateFramebuffers(r.RenderPass, nil)
	if err != nil {
		err = fmt.Errorf("renderer.CreateFramebuffers failed with %s", err)
		return r, err
	}
	vb, err = v.CreateVertexBuffers(gVertexData.Data(), uint32(gVertexData.Sizeof()))
	if err != nil {
		err = fmt.Errorf("renderer.CreateVertexBuffers failed with %s", err)
		return r, err
	}
	ib, err = v.CreateIndexBuffers(gIndexData.Data(), uint32(gIndexData.Sizeof()))
	if err != nil {
		err = fmt.Errorf("renderer.CreateIndexBuffers failed with %s", err)
		return r, err
	}
	gfx, err = createGraphicsPipeline(v.Device, s.DisplaySize, r.RenderPass, s.DescLayout)
	if err != nil {
		err = fmt.Errorf("uniform.createGraphicsPipeline failed with %s", err)
		return r, err
	}
	log.Println("[INFO] swapchain lengths:", s.SwapchainLen)
	err = r.createCommandBuffers(s.DefaultSwapchainLen())
	if err != nil {
		err = fmt.Errorf("uniform.createGraphicsPipeline failed with %s", err)
		return r, err
	}

	vulkanInit()

	return r, nil
}

func UniformDataSize() uint32 {
	return vkTriUniformSize
}

// func NewVulkanDevice(appInfo *vk.ApplicationInfo, window uintptr, instanceExtensions []string, createSurfaceFunc func(interface{}) uintptr) (renderer.VulkanDeviceInfo, error) {
// 	// Phase 1: vk.CreateInstance with vk.InstanceCreateInfo

// 	existingExtensions := getInstanceExtensions()
// 	log.Println("[INFO] Instance extensions:", existingExtensions)

// 	if enableDebug {
// 		instanceExtensions = append(instanceExtensions,
// 			"VK_EXT_debug_report\x00")
// 	}

// 	// ANDROID:
// 	// these layers must be included in APK,
// 	// see Android.mk and ValidationLayers.mk
// 	instanceLayers := []string{
// 		// "VK_LAYER_GOOGLE_threading\x00",
// 		// "VK_LAYER_LUNARG_parameter_validation\x00",
// 		// "VK_LAYER_LUNARG_object_tracker\x00",
// 		// "VK_LAYER_LUNARG_core_validation\x00",
// 		// "VK_LAYER_LUNARG_api_dump\x00",
// 		// "VK_LAYER_LUNARG_image\x00",
// 		// "VK_LAYER_LUNARG_swapchain\x00",
// 		// "VK_LAYER_GOOGLE_unique_objects\x00",
// 	}

// 	instanceCreateInfo := vk.InstanceCreateInfo{
// 		SType:                   vk.StructureTypeInstanceCreateInfo,
// 		PApplicationInfo:        appInfo,
// 		EnabledExtensionCount:   uint32(len(instanceExtensions)),
// 		PpEnabledExtensionNames: instanceExtensions,
// 		EnabledLayerCount:       uint32(len(instanceLayers)),
// 		PpEnabledLayerNames:     instanceLayers,
// 	}
// 	var v renderer.VulkanDeviceInfo
// 	err := vk.Error(vk.CreateInstance(&instanceCreateInfo, nil, &v.Instance))
// 	if err != nil {
// 		err = fmt.Errorf("vk.CreateInstance failed with %s", err)
// 		return v, err
// 	} else {
// 		vk.InitInstance(v.Instance)
// 	}

// 	// Phase 2: vk.CreateAndroidSurface with vk.AndroidSurfaceCreateInfo

// 	v.Surface = vk.SurfaceFromPointer(createSurfaceFunc(v.Instance))
// 	if err != nil {
// 		vk.DestroyInstance(v.Instance, nil)
// 		err = fmt.Errorf("vkCreateWindowSurface failed with %s", err)
// 		return v, err
// 	}
// 	if v.GpuDevices, err = getPhysicalDevices(v.Instance); err != nil {
// 		v.GpuDevices = nil
// 		vk.DestroySurface(v.Instance, v.Surface, nil)
// 		vk.DestroyInstance(v.Instance, nil)
// 		return v, err
// 	}

// 	existingExtensions = getDeviceExtensions(v.GpuDevices[0])
// 	log.Println("[INFO] Device extensions:", existingExtensions)

// 	// Phase 3: vk.CreateDevice with vk.DeviceCreateInfo (a logical device)

// 	// ANDROID:
// 	// these layers must be included in APK,
// 	// see Android.mk and ValidationLayers.mk
// 	deviceLayers := []string{
// 		// "VK_LAYER_GOOGLE_threading\x00",
// 		// "VK_LAYER_LUNARG_parameter_validation\x00",
// 		// "VK_LAYER_LUNARG_object_tracker\x00",
// 		// "VK_LAYER_LUNARG_core_validation\x00",
// 		// "VK_LAYER_LUNARG_api_dump\x00",
// 		// "VK_LAYER_LUNARG_image\x00",
// 		// "VK_LAYER_LUNARG_swapchain\x00",
// 		// "VK_LAYER_GOOGLE_unique_objects\x00",
// 	}

// 	queueCreateInfos := []vk.DeviceQueueCreateInfo{{
// 		SType:            vk.StructureTypeDeviceQueueCreateInfo,
// 		QueueCount:       1,
// 		PQueuePriorities: []float32{1.0},
// 	}}
// 	deviceExtensions := []string{
// 		"VK_KHR_swapchain\x00",
// 	}
// 	deviceCreateInfo := vk.DeviceCreateInfo{
// 		SType:                   vk.StructureTypeDeviceCreateInfo,
// 		QueueCreateInfoCount:    uint32(len(queueCreateInfos)),
// 		PQueueCreateInfos:       queueCreateInfos,
// 		EnabledExtensionCount:   uint32(len(deviceExtensions)),
// 		PpEnabledExtensionNames: deviceExtensions,
// 		EnabledLayerCount:       uint32(len(deviceLayers)),
// 		PpEnabledLayerNames:     deviceLayers,
// 	}
// 	var device vk.Device // we choose the first GPU available for this device
// 	err = vk.Error(vk.CreateDevice(v.GpuDevices[0], &deviceCreateInfo, nil, &device))
// 	if err != nil {
// 		v.GpuDevices = nil
// 		vk.DestroySurface(v.Instance, v.Surface, nil)
// 		vk.DestroyInstance(v.Instance, nil)
// 		err = fmt.Errorf("vk.C	reateDevice failed with %s", err)
// 		return v, err
// 	} else {
// 		v.Device = device
// 		var queue vk.Queue
// 		vk.GetDeviceQueue(device, 0, 0, &queue)
// 		v.Queue = queue
// 	}

// 	if enableDebug {
// 		// Phase 4: vk.CreateDebugReportCallback

// 		dbgCreateInfo := vk.DebugReportCallbackCreateInfo{
// 			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
// 			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
// 			PfnCallback: dbgCallbackFunc,
// 		}
// 		var dbg vk.DebugReportCallback
// 		err = vk.Error(vk.CreateDebugReportCallback(v.Instance, &dbgCreateInfo, nil, &dbg))
// 		if err != nil {
// 			err = fmt.Errorf("vk.CreateDebugReportCallback failed with %s", err)
// 			log.Println("[WARN]", err)
// 			return v, nil
// 		}
// 		v.Dbg = dbg
// 	}
// 	return v, nil
// }

func (gfx *VulkanGfxPipelineInfo) Destroy() {
	if gfx == nil {
		return
	}
	vk.DestroyPipeline(gfx.device, gfx.pipeline, nil)
	vk.DestroyPipelineCache(gfx.device, gfx.pipelineCache, nil)
	vk.DestroyPipelineLayout(gfx.device, gfx.pipelineLayout, nil)
}

func DestroyInOrder(r *VulkanRenderInfo) {

	vk.FreeCommandBuffers(v.Device, r.cmdPool, uint32(len(r.cmdBuffers)), r.cmdBuffers)
	r.cmdBuffers = nil

	vk.DestroyCommandPool(v.Device, r.cmdPool, nil)
	vk.DestroyRenderPass(v.Device, r.RenderPass, nil)

	s.Destroy()
	gfx.Destroy()
	vb.Destroy()
	ib.Destroy()
	vk.DestroyDevice(v.Device, nil)
	if v.Dbg != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(v.Instance, v.Dbg, nil)
	}
	vk.DestroyInstance(v.Instance, nil)
}