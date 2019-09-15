package uniform

import (
	"fmt"
	"log"
	"unsafe"

	"github.com/xlab/linmath"
	vk "github.com/vulkan-go/vulkan"
)

// enableDebug is disabled by default since VK_EXT_debug_report
// is not guaranteed to be present on a device.
// Nvidia Shield K1 fw 1.3.0 lacks this extension,
// on fw 1.2.0 it works fine.
const enableDebug = false

type VulkanDeviceInfo struct {
	gpuDevices []vk.PhysicalDevice

	dbg      vk.DebugReportCallback
	Instance vk.Instance
	Surface  vk.Surface
	Queue    vk.Queue
	Device   vk.Device
}

type VulkanSwapchainInfo struct {
	Device vk.Device

	Swapchains   []vk.Swapchain
	SwapchainLen []uint32

	uniformBuffer []UniformBuffer
	DisplaySize   vk.Extent2D
	DisplayFormat vk.Format

	Framebuffers []vk.Framebuffer
	DisplayViews []vk.ImageView

	DescLayout 		vk.DescriptorSetLayout
	descPool 			vk.DescriptorPool
	descriptorSet	[]vk.DescriptorSet
}

type VulkanRenderInfo struct {
	device vk.Device

	RenderPass vk.RenderPass
	cmdPool    vk.CommandPool
	cmdBuffers []vk.CommandBuffer
	semaphores []vk.Semaphore
	fences     []vk.Fence

	modelMatrix linmath.Mat4x4
	viewMatrix	linmath.Mat4x4
	projectionMatrix linmath.Mat4x4
}

type VulkanBufferInfo struct {
	device    vk.Device
	buffers		[]vk.Buffer
}

type VulkanGfxPipelineInfo struct {
	device vk.Device

	pipelineLayout   vk.PipelineLayout
	pipelineCache    vk.PipelineCache
	pipeline vk.Pipeline
}

func (v *VulkanSwapchainInfo) DefaultSwapchain() vk.Swapchain {
	return v.Swapchains[0]
}

func (v *VulkanSwapchainInfo) DefaultSwapchainLen() uint32 {
	return v.SwapchainLen[0]
}

func (v *VulkanBufferInfo) DefaultBuffer() vk.Buffer {
	return v.buffers[0]
}

func (v *VulkanRenderInfo) DefaultFence() vk.Fence {
	return v.fences[0]
}

func (v *VulkanRenderInfo) DefaultSemaphore() vk.Semaphore {
	return v.semaphores[0]
}

func VulkanInit(v *VulkanDeviceInfo, s *VulkanSwapchainInfo,
	r *VulkanRenderInfo, vb *VulkanBufferInfo, ib *VulkanBufferInfo, gfx *VulkanGfxPipelineInfo) {

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
		check(ret, "vk.BeginCommandBuffer")

		vk.CmdBeginRenderPass(r.cmdBuffers[i], &renderPassBeginInfo, vk.SubpassContentsInline)
		vk.CmdBindPipeline(r.cmdBuffers[i], vk.PipelineBindPointGraphics, gfx.pipeline)
		offsets := make([]vk.DeviceSize, len(vb.buffers))
		// TODO: CmdBindDescriptorSets
		vk.CmdBindDescriptorSets(r.cmdBuffers[i], vk.PipelineBindPointGraphics, gfx.pipelineLayout,
			0, 1, []vk.DescriptorSet{s.descriptorSet[i]}, 0, nil)

		vk.CmdBindVertexBuffers(r.cmdBuffers[i], 0, 1, vb.buffers, offsets)
		vk.CmdBindIndexBuffer(r.cmdBuffers[i], ib.buffers[0], 0, vk.IndexTypeUint16);
		vk.CmdDrawIndexed(r.cmdBuffers[i], (uint32)(len(gIndexData)), 1, 0, 0, 0)
		vk.CmdEndRenderPass(r.cmdBuffers[i])

		ret = vk.EndCommandBuffer(r.cmdBuffers[i])
		check(ret, "vk.EndCommandBuffer")
	}
	fenceCreateInfo := vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
	}
	semaphoreCreateInfo := vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	r.fences = make([]vk.Fence, 1)
	ret := vk.CreateFence(v.Device, &fenceCreateInfo, nil, &r.fences[0])
	check(ret, "vk.CreateFence")
	r.semaphores = make([]vk.Semaphore, 1)
	ret = vk.CreateSemaphore(v.Device, &semaphoreCreateInfo, nil, &r.semaphores[0])
	check(ret, "vk.CreateSemaphore")
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

func CreateGraphicsPipeline(device vk.Device,
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

func VulkanDrawFrame(v VulkanDeviceInfo,
	s VulkanSwapchainInfo, r VulkanRenderInfo) bool {
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

	// // Rotate cube and set uniform buffer
	var MVP linmath.Mat4x4
//	r.modelMatrix.Rotate(&r.modelMatrix, 0.0, 1.0, 0.0, linmath.DegreesToRadians(15))
	MVP.Mult(&r.projectionMatrix, &r.viewMatrix)
	MVP.Mult(&MVP, &r.modelMatrix)
	data := MVP.Data()
	var pData unsafe.Pointer

	vk.MapMemory(v.Device, s.uniformBuffer[nextIdx].Memory, 0, vk.DeviceSize(len(data)), 0, &pData)
	n := vk.Memcopy(pData, data)
	if n != len(data) {
		log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
	}
	vk.UnmapMemory(v.Device, s.uniformBuffer[nextIdx].Memory)

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

func (r *VulkanRenderInfo) CreateCommandBuffers(n uint32) error {
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

func CreateRenderer(device vk.Device, displayFormat vk.Format) (VulkanRenderInfo, error) {
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

	r.projectionMatrix.Perspective(linmath.DegreesToRadians(45.0), 1.0, 0.1, 100.0);
	r.viewMatrix.LookAt(eyeVec, origin, upVec)
	r.modelMatrix.Identity()
	r.projectionMatrix[1][1] *= -1 // Flip projection matrix from GL to Vulkan orientation.

	r.device = device
	return r, nil
}

func NewVulkanDevice(appInfo *vk.ApplicationInfo, window uintptr, instanceExtensions []string, createSurfaceFunc func(interface{}) uintptr) (VulkanDeviceInfo, error) {
	// Phase 1: vk.CreateInstance with vk.InstanceCreateInfo

	existingExtensions := getInstanceExtensions()
	log.Println("[INFO] Instance extensions:", existingExtensions)

	if enableDebug {
		instanceExtensions = append(instanceExtensions,
			"VK_EXT_debug_report\x00")
	}

	// ANDROID:
	// these layers must be included in APK,
	// see Android.mk and ValidationLayers.mk
	instanceLayers := []string{
		// "VK_LAYER_GOOGLE_threading\x00",
		// "VK_LAYER_LUNARG_parameter_validation\x00",
		// "VK_LAYER_LUNARG_object_tracker\x00",
		// "VK_LAYER_LUNARG_core_validation\x00",
		// "VK_LAYER_LUNARG_api_dump\x00",
		// "VK_LAYER_LUNARG_image\x00",
		// "VK_LAYER_LUNARG_swapchain\x00",
		// "VK_LAYER_GOOGLE_unique_objects\x00",
	}

	instanceCreateInfo := vk.InstanceCreateInfo{
		SType:                   vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo:        appInfo,
		EnabledExtensionCount:   uint32(len(instanceExtensions)),
		PpEnabledExtensionNames: instanceExtensions,
		EnabledLayerCount:       uint32(len(instanceLayers)),
		PpEnabledLayerNames:     instanceLayers,
	}
	var v VulkanDeviceInfo
	err := vk.Error(vk.CreateInstance(&instanceCreateInfo, nil, &v.Instance))
	if err != nil {
		err = fmt.Errorf("vk.CreateInstance failed with %s", err)
		return v, err
	} else {
		vk.InitInstance(v.Instance)
	}

	// Phase 2: vk.CreateAndroidSurface with vk.AndroidSurfaceCreateInfo

	v.Surface = vk.SurfaceFromPointer(createSurfaceFunc(v.Instance))
	if err != nil {
		vk.DestroyInstance(v.Instance, nil)
		err = fmt.Errorf("vkCreateWindowSurface failed with %s", err)
		return v, err
	}
	if v.gpuDevices, err = getPhysicalDevices(v.Instance); err != nil {
		v.gpuDevices = nil
		vk.DestroySurface(v.Instance, v.Surface, nil)
		vk.DestroyInstance(v.Instance, nil)
		return v, err
	}

	existingExtensions = getDeviceExtensions(v.gpuDevices[0])
	log.Println("[INFO] Device extensions:", existingExtensions)

	// Phase 3: vk.CreateDevice with vk.DeviceCreateInfo (a logical device)

	// ANDROID:
	// these layers must be included in APK,
	// see Android.mk and ValidationLayers.mk
	deviceLayers := []string{
		// "VK_LAYER_GOOGLE_threading\x00",
		// "VK_LAYER_LUNARG_parameter_validation\x00",
		// "VK_LAYER_LUNARG_object_tracker\x00",
		// "VK_LAYER_LUNARG_core_validation\x00",
		// "VK_LAYER_LUNARG_api_dump\x00",
		// "VK_LAYER_LUNARG_image\x00",
		// "VK_LAYER_LUNARG_swapchain\x00",
		// "VK_LAYER_GOOGLE_unique_objects\x00",
	}

	queueCreateInfos := []vk.DeviceQueueCreateInfo{{
		SType:            vk.StructureTypeDeviceQueueCreateInfo,
		QueueCount:       1,
		PQueuePriorities: []float32{1.0},
	}}
	deviceExtensions := []string{
		"VK_KHR_swapchain\x00",
	}
	deviceCreateInfo := vk.DeviceCreateInfo{
		SType:                   vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount:    uint32(len(queueCreateInfos)),
		PQueueCreateInfos:       queueCreateInfos,
		EnabledExtensionCount:   uint32(len(deviceExtensions)),
		PpEnabledExtensionNames: deviceExtensions,
		EnabledLayerCount:       uint32(len(deviceLayers)),
		PpEnabledLayerNames:     deviceLayers,
	}
	var device vk.Device // we choose the first GPU available for this device
	err = vk.Error(vk.CreateDevice(v.gpuDevices[0], &deviceCreateInfo, nil, &device))
	if err != nil {
		v.gpuDevices = nil
		vk.DestroySurface(v.Instance, v.Surface, nil)
		vk.DestroyInstance(v.Instance, nil)
		err = fmt.Errorf("vk.CreateDevice failed with %s", err)
		return v, err
	} else {
		v.Device = device
		var queue vk.Queue
		vk.GetDeviceQueue(device, 0, 0, &queue)
		v.Queue = queue
	}

	if enableDebug {
		// Phase 4: vk.CreateDebugReportCallback

		dbgCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
			PfnCallback: dbgCallbackFunc,
		}
		var dbg vk.DebugReportCallback
		err = vk.Error(vk.CreateDebugReportCallback(v.Instance, &dbgCreateInfo, nil, &dbg))
		if err != nil {
			err = fmt.Errorf("vk.CreateDebugReportCallback failed with %s", err)
			log.Println("[WARN]", err)
			return v, nil
		}
		v.dbg = dbg
	}
	return v, nil
}

func dbgCallbackFunc(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {

	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		log.Printf("[ERROR %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		log.Printf("[WARN %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	default:
		log.Printf("[WARN] unknown debug message %d (layer %s)", messageCode, pLayerPrefix)
	}
	return vk.Bool32(vk.False)
}

func getPhysicalDevices(instance vk.Instance) ([]vk.PhysicalDevice, error) {
	var gpuCount uint32
	err := vk.Error(vk.EnumeratePhysicalDevices(instance, &gpuCount, nil))
	if err != nil {
		err = fmt.Errorf("vk.EnumeratePhysicalDevices failed with %s", err)
		return nil, err
	}
	if gpuCount == 0 {
		err = fmt.Errorf("getPhysicalDevice: no GPUs found on the system")
		return nil, err
	}
	gpuList := make([]vk.PhysicalDevice, gpuCount)
	err = vk.Error(vk.EnumeratePhysicalDevices(instance, &gpuCount, gpuList))
	if err != nil {
		err = fmt.Errorf("vk.EnumeratePhysicalDevices failed with %s", err)
		return nil, err
	}
	return gpuList, nil
}

func getDeviceExtensions(gpu vk.PhysicalDevice) (extNames []string) {
	var deviceExtLen uint32
	ret := vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, nil)
	check(ret, "vk.EnumerateDeviceExtensionProperties")
	deviceExt := make([]vk.ExtensionProperties, deviceExtLen)
	ret = vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, deviceExt)
	check(ret, "vk.EnumerateDeviceExtensionProperties")
	for _, ext := range deviceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
}

func getInstanceExtensions() (extNames []string) {
	var instanceExtLen uint32
	ret := vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, nil)
	check(ret, "vk.EnumerateInstanceExtensionProperties")
	instanceExt := make([]vk.ExtensionProperties, instanceExtLen)
	ret = vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, instanceExt)
	check(ret, "vk.EnumerateInstanceExtensionProperties")
	for _, ext := range instanceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
}

func (v VulkanDeviceInfo) CreateUniformBuffers() (*UniformBuffer, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	var MVP linmath.Mat4x4
	uniformData := vkTriUniform{
		mvp: MVP,
	}
	dataRaw := uniformData.Data()

	//queueFamilyIdx := []uint32{0}
	uniformBufferCreateInfo := vk.BufferCreateInfo{
		SType:                 vk.StructureTypeBufferCreateInfo,
		Size:                  vk.DeviceSize(len(dataRaw)),
		Usage:                 vk.BufferUsageFlags(vk.BufferUsageUniformBufferBit),
	//	SharingMode:           vk.SharingModeExclusive,
	//	QueueFamilyIndexCount: 1,
	//	PQueueFamilyIndices:   queueFamilyIdx,
	}

	uniformBuffer := VulkanBufferInfo{
		buffers: make([]vk.Buffer, 1),
	}
	var uniformDeviceMemory vk.DeviceMemory
	err := vk.Error(vk.CreateBuffer(v.Device, &uniformBufferCreateInfo, nil, &uniformBuffer.buffers[0]))
	if err != nil {
		err = fmt.Errorf("vk.CreateBuffer failed with %s", err)
		return nil, err
	}

	// Phase 2: vk.GetBufferMemoryRequirements
	//			vk.FindMemoryTypeIndex
	// 			assign a proper memory type for that buffer

	var memReq vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(v.Device, uniformBuffer.DefaultBuffer(), &memReq)
	memReq.Deref()
	allocInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReq.Size,
		MemoryTypeIndex: 0, // see below
	}
	allocInfo.MemoryTypeIndex, _ = vk.FindMemoryTypeIndex(gpu, memReq.MemoryTypeBits,
		vk.MemoryPropertyHostVisibleBit)

	// Phase 3: vk.AllocateMemory
	//			vk.MapMemory
	//			vk.MemCopyFloat32
	//			vk.UnmapMemory
	// 			allocate and map memory for that buffer

	err = vk.Error(vk.AllocateMemory(v.Device, &allocInfo, nil, &uniformDeviceMemory))
	if err != nil {
		err = fmt.Errorf("vk.AllocateMemory failed with %s", err)
		return nil, err
	}
	var uniformDataPtr unsafe.Pointer
	vk.MapMemory(v.Device, uniformDeviceMemory, 0, vk.DeviceSize(len(dataRaw)), 0, &uniformDataPtr)
	n := vk.Memcopy(uniformDataPtr, dataRaw)
	if n != len(dataRaw) {
		log.Println("[WARN] failed to copy uniform buffer data")
	}
	vk.UnmapMemory(v.Device, uniformDeviceMemory)

	// Phase 4: vk.BindBufferMemory
	//			copy vertex data and bind buffer

	err = vk.Error(vk.BindBufferMemory(v.Device, uniformBuffer.DefaultBuffer(), uniformDeviceMemory, 0))
	if err != nil {
		err = fmt.Errorf("vk.BindBufferMemory failed with %s", err)
		return nil, err
	}
//	uniformBuffer.device = v.Device

	buffer := &UniformBuffer{
		// Device: v.Device,
		Buffer: uniformBuffer.DefaultBuffer(),
		Memory: uniformDeviceMemory,
	}

	return buffer, err
}

func (v *VulkanDeviceInfo) CreateSwapchain() (VulkanSwapchainInfo, error) {
	gpu := v.gpuDevices[0]

	var s VulkanSwapchainInfo
	var descLayout vk.DescriptorSetLayout
	ret := vk.CreateDescriptorSetLayout(v.Device, &vk.DescriptorSetLayoutCreateInfo{
		SType:		vk.StructureTypeDescriptorSetLayoutCreateInfo,
		BindingCount: 1,
		PBindings: []vk.DescriptorSetLayoutBinding{
		{
			Binding: 0,
			DescriptorType: vk.DescriptorTypeUniformBuffer,
			DescriptorCount: 1,
			StageFlags: vk.ShaderStageFlags(vk.ShaderStageVertexBit),
		}},
	}, nil, &descLayout)
	s.DescLayout = descLayout
	err := vk.Error(ret)
	if err != nil {
		err = fmt.Errorf("vk.CreateDescriptorSetLayout failed with %s", err)
		return s, err
	}

	// Phase 1: vk.GetPhysicalDeviceSurfaceCapabilities
	//			vk.GetPhysicalDeviceSurfaceFormats
	var surfaceCapabilities vk.SurfaceCapabilities
	err = vk.Error(vk.GetPhysicalDeviceSurfaceCapabilities(gpu, v.Surface, &surfaceCapabilities))
	if err != nil {
		err = fmt.Errorf("vk.GetPhysicalDeviceSurfaceCapabilities failed with %s", err)
		return s, err
	}
	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(gpu, v.Surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(gpu, v.Surface, &formatCount, formats)

	log.Println("[INFO] got", formatCount, "physical device surface formats")

	chosenFormat := -1
	for i := 0; i < int(formatCount); i++ {
		formats[i].Deref()
		if formats[i].Format == vk.FormatB8g8r8a8Unorm ||
			formats[i].Format == vk.FormatR8g8b8a8Unorm {
			chosenFormat = i
			break
		}
	}
	if chosenFormat < 0 {
		err := fmt.Errorf("vk.GetPhysicalDeviceSurfaceFormats not found suitable format")
		return s, err
	}

	// Phase 2: vk.CreateSwapchain
	//			create a swapchain with supported capabilities and format

	surfaceCapabilities.Deref()
	s.DisplaySize = surfaceCapabilities.CurrentExtent
	s.DisplaySize.Deref()
	s.DisplayFormat = formats[chosenFormat].Format
	queueFamily := []uint32{0}
	swapchainCreateInfo := vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         v.Surface,
		MinImageCount:   surfaceCapabilities.MinImageCount,
		ImageFormat:     formats[chosenFormat].Format,
		ImageColorSpace: formats[chosenFormat].ColorSpace,
		ImageExtent:     surfaceCapabilities.CurrentExtent,
		ImageUsage:      vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		PreTransform:    vk.SurfaceTransformIdentityBit,

		ImageArrayLayers:      1,
		ImageSharingMode:      vk.SharingModeExclusive,
		QueueFamilyIndexCount: 1,
		PQueueFamilyIndices:   queueFamily,
		PresentMode:           vk.PresentModeFifo,
		OldSwapchain:          vk.NullSwapchain,
		Clipped:               vk.False,
	}
	s.Swapchains = make([]vk.Swapchain, 1)
	err = vk.Error(vk.CreateSwapchain(v.Device, &swapchainCreateInfo, nil, &s.Swapchains[0]))
	if err != nil {
		err = fmt.Errorf("vk.CreateSwapchain failed with %s", err)
		return s, err
	}
	s.SwapchainLen = make([]uint32, 1)
	err = vk.Error(vk.GetSwapchainImages(v.Device, s.DefaultSwapchain(), &s.SwapchainLen[0], nil))
	if err != nil {
		err = fmt.Errorf("vk.GetSwapchainImages failed with %s", err)
		return s, err
	}
	var imageCount uint32 = s.SwapchainLen[0];
	s.uniformBuffer = make([]UniformBuffer, imageCount)
	// create uniform buffer.
	for i := uint32(0); i < imageCount; i++ {
		buffer, err := v.CreateUniformBuffers();
		s.uniformBuffer[i].Buffer = buffer.Buffer;
		s.uniformBuffer[i].Memory = buffer.Memory;
		orPanic(err)
	}

	// // TODO: 	s.prepareDescriptorPool()
	// //	vk.CreateDescriptorPool
  // var descPool vk.DescriptorPool
	// ret := vk.CreateDescriptorPool(dev, &vk.DescriptorPoolCreateInfo{
	// 	SType:         vk.StructureTypeDescriptorPoolCreateInfo,
	// 	MaxSets:       uint32(s.SwapchainLen[0]),
	// 	PoolSizeCount: 2,
	// 	PPoolSizes: []vk.DescriptorPoolSize{{
	// 		Type:            vk.DescriptorTypeUniformBuffer,
	// 		DescriptorCount: uint32(len(swapchainImageResources)),
	// 	}, {
	// 		Type:            vk.DescriptorTypeCombinedImageSampler,
	// 		DescriptorCount: uint32(len(swapchainImageResources) * len(texEnabled)),
	// 	}},
	// }, nil, &descPool)
	// orPanic(as.NewError(ret))
	// s.descPool = descPool


	// Phase 6:
	//     s.prepareDescriptorSet()
	//			vk.AllocateDescriptorSets

	for i := range formats {
		formats[i].Free()
	}
	s.Device = v.Device
	return s, nil
}

func (v VulkanDeviceInfo) CreateVertexBuffers() (VulkanBufferInfo, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	queueFamilyIdx := []uint32{0}
	vertexBufferCreateInfo := vk.BufferCreateInfo{
		SType:                 vk.StructureTypeBufferCreateInfo,
		Size:                  vk.DeviceSize(gVertexData.Sizeof()),
		Usage:                 vk.BufferUsageFlags(vk.BufferUsageVertexBufferBit),
		SharingMode:           vk.SharingModeExclusive,
		QueueFamilyIndexCount: 1,
		PQueueFamilyIndices:   queueFamilyIdx,
	}
	vertexBuffer := VulkanBufferInfo{
		buffers: make([]vk.Buffer, 1),
	}
	err := vk.Error(vk.CreateBuffer(v.Device, &vertexBufferCreateInfo, nil, &vertexBuffer.buffers[0]))
	if err != nil {
		err = fmt.Errorf("vk.CreateBuffer failed with %s", err)
		return vertexBuffer, err
	}

	// Phase 2: vk.GetBufferMemoryRequirements
	//			vk.FindMemoryTypeIndex
	// 			assign a proper memory type for that buffer

	var memReq vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(v.Device, vertexBuffer.DefaultBuffer(), &memReq)
	memReq.Deref()
	allocInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReq.Size,
		MemoryTypeIndex: 0, // see below
	}
	allocInfo.MemoryTypeIndex, _ = vk.FindMemoryTypeIndex(gpu, memReq.MemoryTypeBits,
		vk.MemoryPropertyHostVisibleBit)

	// Phase 3: vk.AllocateMemory
	//			vk.MapMemory
	//			vk.MemCopyFloat32
	//			vk.UnmapMemory
	// 			allocate and map memory for that buffer

	var vertexDeviceMemory vk.DeviceMemory
	err = vk.Error(vk.AllocateMemory(v.Device, &allocInfo, nil, &vertexDeviceMemory))
	if err != nil {
		err = fmt.Errorf("vk.AllocateMemory failed with %s", err)
		return vertexBuffer, err
	}
	var vertexDataPtr unsafe.Pointer
	vk.MapMemory(v.Device, vertexDeviceMemory, 0, vk.DeviceSize(gVertexData.Sizeof()), 0, &vertexDataPtr)
	n := vk.Memcopy(vertexDataPtr, gVertexData.Data())
	if n != gVertexData.Sizeof() {
		log.Println("[WARN] failed to copy vertex buffer data")
	}
	vk.UnmapMemory(v.Device, vertexDeviceMemory)

	// Phase 4: vk.BindBufferMemory
	//			copy vertex data and bind buffer

	err = vk.Error(vk.BindBufferMemory(v.Device, vertexBuffer.DefaultBuffer(), vertexDeviceMemory, 0))
	if err != nil {
		err = fmt.Errorf("vk.BindBufferMemory failed with %s", err)
		return vertexBuffer, err
	}
	vertexBuffer.device = v.Device
	return vertexBuffer, err
}

func (v VulkanDeviceInfo) CreateIndexBuffers() (VulkanBufferInfo, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	queueFamilyIdx := []uint32{0}
	indexBufferCreateInfo := vk.BufferCreateInfo{
		SType:                 vk.StructureTypeBufferCreateInfo,
		Size:                  vk.DeviceSize(gIndexData.Sizeof()),
		Usage:                 vk.BufferUsageFlags(vk.BufferUsageVertexBufferBit),
		SharingMode:           vk.SharingModeExclusive,
		QueueFamilyIndexCount: 1,
		PQueueFamilyIndices:   queueFamilyIdx,
	}
	indexBuffer := VulkanBufferInfo{
		buffers: make([]vk.Buffer, 1),
	}
	err := vk.Error(vk.CreateBuffer(v.Device, &indexBufferCreateInfo, nil, &indexBuffer.buffers[0]))
	if err != nil {
		err = fmt.Errorf("vk.CreateBuffer failed with %s", err)
		return indexBuffer, err
	}

	// Phase 2: vk.GetBufferMemoryRequirements
	//			vk.FindMemoryTypeIndex
	// 			assign a proper memory type for that buffer

	var memReq vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(v.Device, indexBuffer.DefaultBuffer(), &memReq)
	memReq.Deref()
	allocInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReq.Size,
		MemoryTypeIndex: 0, // see below
	}
	allocInfo.MemoryTypeIndex, _ = vk.FindMemoryTypeIndex(gpu, memReq.MemoryTypeBits,
		vk.MemoryPropertyHostVisibleBit)

	// Phase 3: vk.AllocateMemory
	//			vk.MapMemory
	//			vk.MemCopyFloat32
	//			vk.UnmapMemory
	// 			allocate and map memory for that buffer

	var indexDeviceMemory vk.DeviceMemory
	err = vk.Error(vk.AllocateMemory(v.Device, &allocInfo, nil, &indexDeviceMemory))
	if err != nil {
		err = fmt.Errorf("vk.AllocateMemory failed with %s", err)
		return indexBuffer, err
	}
	var indexDataPtr unsafe.Pointer
	vk.MapMemory(v.Device, indexDeviceMemory, 0, vk.DeviceSize(gIndexData.Sizeof()), 0, &indexDataPtr)
	n := vk.Memcopy(indexDataPtr, gIndexData.Data())
	if n != gIndexData.Sizeof() {
		log.Println("[WARN] failed to copy index buffer data")
	}
	vk.UnmapMemory(v.Device, indexDeviceMemory)

	// Phase 4: vk.BindBufferMemory
	//			copy vertex data and bind buffer

	err = vk.Error(vk.BindBufferMemory(v.Device, indexBuffer.DefaultBuffer(), indexDeviceMemory, 0))
	if err != nil {
		err = fmt.Errorf("vk.BindBufferMemory failed with %s", err)
		return indexBuffer, err
	}
	indexBuffer.device = v.Device
	return indexBuffer, err
}


type UniformBuffer struct {
	// device for destroy purposes.
	//Device vk.Device
	// Buffer is the buffer object.
	Buffer vk.Buffer
	// Memory is the device memory backing buffer object.
	Memory vk.DeviceMemory
}

func (s *VulkanSwapchainInfo) CreateDescriptorPool() error {
	dev := s.Device
	var descPool vk.DescriptorPool
	ret := vk.CreateDescriptorPool(dev, &vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		MaxSets:       uint32(s.SwapchainLen[0]),
		PoolSizeCount: 1,
		PPoolSizes: []vk.DescriptorPoolSize{{
			Type:            vk.DescriptorTypeUniformBuffer,
			DescriptorCount: uint32(s.SwapchainLen[0]),
		}},
	}, nil, &descPool)
	// orPanic(as.NewError(ret))
	err := vk.Error(ret)
	if (err != nil) {
		return fmt.Errorf("vk.CreateDescriptorPool failed with %s", err)
	}

	s.descPool = descPool
	return nil
}

func (s *VulkanSwapchainInfo) CreateDescriptorSet() error{
	dev := s.Device
//	swapchainImageResources := s.Context().SwapchainImageResources()

	// texInfos := make([]vk.DescriptorImageInfo, 0, len(s.textures))
	// for _, tex := range s.textures {
	// 	texInfos = append(texInfos, vk.DescriptorImageInfo{
	// 		Sampler:     tex.sampler,
	// 		ImageView:   tex.view,
	// 		ImageLayout: vk.ImageLayoutGeneral,
	// 	})
	// }

	s.descriptorSet = make([]vk.DescriptorSet, s.SwapchainLen[0])
	for i := uint32(0); i < s.SwapchainLen[0]; i++ {
	//for _, res := range swapchainImageResources {
		var set vk.DescriptorSet
		ret := vk.AllocateDescriptorSets(dev, &vk.DescriptorSetAllocateInfo{
			SType:              vk.StructureTypeDescriptorSetAllocateInfo,
			DescriptorPool:     s.descPool,
			DescriptorSetCount: 1,
			PSetLayouts:        []vk.DescriptorSetLayout{s.DescLayout},
		}, &set)
		err := vk.Error(ret)
		if (err != nil) {
			return fmt.Errorf("vk.AllocateDescriptorSets failed with %s", err)
		}

		s.descriptorSet[i] = set

		vk.UpdateDescriptorSets(dev, 1, []vk.WriteDescriptorSet{{
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstSet:          set,
			DescriptorCount: 1,
			DescriptorType:  vk.DescriptorTypeUniformBuffer,
			PBufferInfo: []vk.DescriptorBufferInfo{{
				Offset: 0,
				Range:  vk.DeviceSize(vkTriUniformSize),
				Buffer: s.uniformBuffer[i].Buffer,
			}},
		}}, 0, nil)
	}
	return nil
}

func (s *VulkanSwapchainInfo) CreateFramebuffers(renderPass vk.RenderPass, depthView vk.ImageView) error {
	// Phase 1: vk.GetSwapchainImages

	var swapchainImagesCount uint32
	err := vk.Error(vk.GetSwapchainImages(s.Device, s.DefaultSwapchain(), &swapchainImagesCount, nil))
	if err != nil {
		err = fmt.Errorf("vk.GetSwapchainImages failed with %s", err)
		return err
	}
	swapchainImages := make([]vk.Image, swapchainImagesCount)
	vk.GetSwapchainImages(s.Device, s.DefaultSwapchain(), &swapchainImagesCount, swapchainImages)

	// Phase 2: vk.CreateImageView
	//			create image view for each swapchain image

	s.DisplayViews = make([]vk.ImageView, len(swapchainImages))
	for i := range s.DisplayViews {
		viewCreateInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Image:    swapchainImages[i],
			ViewType: vk.ImageViewType2d,
			Format:   s.DisplayFormat,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			},
		}
		err := vk.Error(vk.CreateImageView(s.Device, &viewCreateInfo, nil, &s.DisplayViews[i]))
		if err != nil {
			err = fmt.Errorf("vk.CreateImageView failed with %s", err)
			return err // bail out
		}
	}
	swapchainImages = nil

	// Phase 3: vk.CreateFramebuffer
	//			create a framebuffer from each swapchain image

	s.Framebuffers = make([]vk.Framebuffer, s.DefaultSwapchainLen())
	for i := range s.Framebuffers {
		attachments := []vk.ImageView{
			s.DisplayViews[i], depthView,
		}
		fbCreateInfo := vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			RenderPass:      renderPass,
			Layers:          1,
			AttachmentCount: 1, // 2 if has depthView
			PAttachments:    attachments,
			Width:           s.DisplaySize.Width,
			Height:          s.DisplaySize.Height,
		}
		if depthView != vk.NullImageView {
			fbCreateInfo.AttachmentCount = 2
		}
		err := vk.Error(vk.CreateFramebuffer(s.Device, &fbCreateInfo, nil, &s.Framebuffers[i]))
		if err != nil {
			err = fmt.Errorf("vk.CreateFramebuffer failed with %s", err)
			return err // bail out
		}
	}
	return nil
}

func (buf *VulkanBufferInfo) Destroy() {
	for i := range buf.buffers {
		vk.DestroyBuffer(buf.device, buf.buffers[i], nil)
	}
}

func (gfx *VulkanGfxPipelineInfo) Destroy() {
	if gfx == nil {
		return
	}
	vk.DestroyPipeline(gfx.device, gfx.pipeline, nil)
	vk.DestroyPipelineCache(gfx.device, gfx.pipelineCache, nil)
	vk.DestroyPipelineLayout(gfx.device, gfx.pipelineLayout, nil)
	// vk.DestroyDescriptorSetLayout(gfx.device, gfx.descLayout, nil)
	// TODO destroy descripterPool, desriptorSet, descLayout
}

func (s *VulkanSwapchainInfo) Destroy() {
	for i := uint32(0); i < s.DefaultSwapchainLen(); i++ {
		vk.DestroyFramebuffer(s.Device, s.Framebuffers[i], nil)
		vk.DestroyImageView(s.Device, s.DisplayViews[i], nil)
	}
	// TODO: Destroy ub.

	s.Framebuffers = nil
	s.DisplayViews = nil
	for i := range s.Swapchains {
		vk.DestroySwapchain(s.Device, s.Swapchains[i], nil)
	}
}

func DestroyInOrder(v *VulkanDeviceInfo, s *VulkanSwapchainInfo,
	r *VulkanRenderInfo, vb *VulkanBufferInfo, ib *VulkanBufferInfo, gfx *VulkanGfxPipelineInfo) {

	vk.FreeCommandBuffers(v.Device, r.cmdPool, uint32(len(r.cmdBuffers)), r.cmdBuffers)
	r.cmdBuffers = nil

	vk.DestroyCommandPool(v.Device, r.cmdPool, nil)
	vk.DestroyRenderPass(v.Device, r.RenderPass, nil)

	s.Destroy()
	gfx.Destroy()
	vb.Destroy()
	ib.Destroy()
	// ub.Destroy()
	vk.DestroyDevice(v.Device, nil)
	if v.dbg != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(v.Instance, v.dbg, nil)
	}
	vk.DestroyInstance(v.Instance, nil)
}