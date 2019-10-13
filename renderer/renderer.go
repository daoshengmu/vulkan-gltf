package renderer

import (
	"fmt"
	"log"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
	"github.com/vulkan-gltf/util"
)

// enableDebug is disabled by default since VK_EXT_debug_report
// is not guaranteed to be present on a device.
// Nvidia Shield K1 fw 1.3.0 lacks this extension,
// on fw 1.2.0 it works fine.
const enableDebug = false

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
		err = fmt.Errorf("vk.C	reateDevice failed with %s", err)
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
		v.Dbg = dbg
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

func getDeviceExtensions(gpu vk.PhysicalDevice) (extNames []string) {
	var deviceExtLen uint32
	ret := vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, nil)
	util.Check(ret, "vk.EnumerateDeviceExtensionProperties")
	deviceExt := make([]vk.ExtensionProperties, deviceExtLen)
	ret = vk.EnumerateDeviceExtensionProperties(gpu, "", &deviceExtLen, deviceExt)
	util.Check(ret, "vk.EnumerateDeviceExtensionProperties")
	for _, ext := range deviceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
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

func getInstanceExtensions() (extNames []string) {
	var instanceExtLen uint32
	ret := vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, nil)
	util.Check(ret, "vk.EnumerateInstanceExtensionProperties")
	instanceExt := make([]vk.ExtensionProperties, instanceExtLen)
	ret = vk.EnumerateInstanceExtensionProperties("", &instanceExtLen, instanceExt)
	util.Check(ret, "vk.EnumerateInstanceExtensionProperties")
	for _, ext := range instanceExt {
		ext.Deref()
		extNames = append(extNames,
			vk.ToString(ext.ExtensionName[:]))
	}
	return extNames
}

type VulkanDeviceInfo struct {
	gpuDevices []vk.PhysicalDevice

	Dbg      vk.DebugReportCallback
	Instance vk.Instance
	Surface  vk.Surface
	Queue    vk.Queue
	Device   vk.Device
}

type VulkanSwapchainInfo struct {
	Device vk.Device

	Swapchains   []vk.Swapchain
	SwapchainLen []uint32

	UniformBuffer []UniformBuffer
	DisplaySize   vk.Extent2D
	DisplayFormat vk.Format

	Framebuffers []vk.Framebuffer
	DisplayViews []vk.ImageView

	DescLayout 		vk.DescriptorSetLayout
	DescPool 			vk.DescriptorPool
	DescriptorSet	[]vk.DescriptorSet
}

func (v *VulkanSwapchainInfo) DefaultSwapchain() vk.Swapchain {
	return v.Swapchains[0]
}

func (v *VulkanSwapchainInfo) DefaultSwapchainLen() uint32 {
	return v.SwapchainLen[0]
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
	err := vk.Error(ret)
	if (err != nil) {
		return fmt.Errorf("vk.CreateDescriptorPool failed with %s", err)
	}

	s.DescPool = descPool
	return nil
}

func (s *VulkanSwapchainInfo) CreateDescriptorSet(uniformSize vk.DeviceSize) error{
	dev := s.Device

	s.DescriptorSet = make([]vk.DescriptorSet, s.SwapchainLen[0])
	for i := uint32(0); i < s.SwapchainLen[0]; i++ {
		var set vk.DescriptorSet
		ret := vk.AllocateDescriptorSets(dev, &vk.DescriptorSetAllocateInfo{
			SType:              vk.StructureTypeDescriptorSetAllocateInfo,
			DescriptorPool:     s.DescPool,
			DescriptorSetCount: 1,
			PSetLayouts:        []vk.DescriptorSetLayout{s.DescLayout},
		}, &set)
		err := vk.Error(ret)
		if (err != nil) {
			return fmt.Errorf("vk.AllocateDescriptorSets failed with %s", err)
		}

		s.DescriptorSet[i] = set

		vk.UpdateDescriptorSets(dev, 1, []vk.WriteDescriptorSet{{
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstSet:          set,
			DescriptorCount: 1,
			DescriptorType:  vk.DescriptorTypeUniformBuffer,
			PBufferInfo: []vk.DescriptorBufferInfo{{
				Offset: 0,
				Range:  uniformSize,//vk.DeviceSize(vkTriUniformSize),
				Buffer: s.UniformBuffer[i].buffer,
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

func (s *VulkanSwapchainInfo) Destroy() {
	for i := uint32(0); i < s.DefaultSwapchainLen(); i++ {
		vk.DestroyBuffer(s.Device, s.UniformBuffer[i].buffer, nil)
		vk.FreeMemory(s.Device, s.UniformBuffer[i].memory, nil)
		vk.DestroyFramebuffer(s.Device, s.Framebuffers[i], nil)
		vk.DestroyImageView(s.Device, s.DisplayViews[i], nil)
		vk.FreeDescriptorSets(s.Device, s.DescPool, i, &s.DescriptorSet[i])
	}

	vk.DestroyDescriptorSetLayout(s.Device, s.DescLayout, nil)
	vk.DestroyDescriptorPool(s.Device, s.DescPool, nil)

	s.Framebuffers = nil
	s.DisplayViews = nil
	for i := range s.Swapchains {
		vk.DestroySwapchain(s.Device, s.Swapchains[i], nil)
	}
}

type VulkanBufferInfo struct {
	device    vk.Device
	buffers		[]vk.Buffer
}

func (v *VulkanBufferInfo) GetDevice() vk.Device {
	return v.device
}

func (v *VulkanBufferInfo) GetBufferLen() int {
	return len(v.buffers)
}

func (v *VulkanBufferInfo) GetBuffers() *[]vk.Buffer {
	return &v.buffers
}

func (v *VulkanBufferInfo) DefaultBuffer() vk.Buffer {
	return v.buffers[0]
}

func (buf *VulkanBufferInfo) Destroy() {
	for i := range buf.buffers {
		vk.DestroyBuffer(buf.device, buf.buffers[i], nil)
	}
}

type UniformBuffer struct {
	// device for destroy purposes.
	// Buffer is the buffer object.
	buffer vk.Buffer
	// Memory is the device memory backing buffer object.
	memory vk.DeviceMemory
}

func (buf *UniformBuffer) GetMemory() vk.DeviceMemory {
	return buf.memory;
}

func (v VulkanDeviceInfo) CreateUniformBuffers(uniformData []byte) (*UniformBuffer, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	//var MVP linmath.Mat4x4
	// uniformData := vkTriUniform{
	// 	mvp: MVP,
	// }
	dataRaw := uniformData;//uniformData.Data()

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

	buffer := &UniformBuffer{
		buffer: uniformBuffer.DefaultBuffer(),
		memory: uniformDeviceMemory,
	}

	return buffer, err
}

func (v *VulkanDeviceInfo) CreateSwapchain(uniformData []byte) (VulkanSwapchainInfo, error) {
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
	s.UniformBuffer = make([]UniformBuffer, imageCount)

	// create uniform buffer.
	for i := uint32(0); i < imageCount; i++ {
		buffer, err := v.CreateUniformBuffers(uniformData);
		s.UniformBuffer[i].buffer = buffer.buffer;
		s.UniformBuffer[i].memory = buffer.memory;
		util.OrPanic(err)
	}

	for i := range formats {
		formats[i].Free()
	}
	s.Device = v.Device
	return s, nil
}

func (v VulkanDeviceInfo) CreateVertexBuffers(data []byte, size uint32) (VulkanBufferInfo, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	queueFamilyIdx := []uint32{0}
	vertexBufferCreateInfo := vk.BufferCreateInfo{
		SType:                 vk.StructureTypeBufferCreateInfo,
		Size:                  vk.DeviceSize(size),//gVertexData.Sizeof()),
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
	vk.MapMemory(v.Device, vertexDeviceMemory, 0, vk.DeviceSize(size), 0, &vertexDataPtr)
	// vk.MapMemory(v.Device, vertexDeviceMemory, 0, vk.DeviceSize(gVertexData.Sizeof()), 0, &vertexDataPtr)
	n := vk.Memcopy(vertexDataPtr, data)//gVertexData.Data())
	if n != int(size) {//gVertexData.Sizeof() {
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

func (v VulkanDeviceInfo) CreateIndexBuffers(data []byte, size uint32) (VulkanBufferInfo, error) {
	gpu := v.gpuDevices[0]

	// Phase 1: vk.CreateBuffer
	//			create the triangle vertex buffer
	queueFamilyIdx := []uint32{0}
	indexBufferCreateInfo := vk.BufferCreateInfo{
		SType:                 vk.StructureTypeBufferCreateInfo,
		Size:                  vk.DeviceSize(size),//vk.DeviceSize(gIndexData.Sizeof()),
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
	vk.MapMemory(v.Device, indexDeviceMemory, 0, vk.DeviceSize(size), 0, &indexDataPtr)
	// vk.MapMemory(v.Device, indexDeviceMemory, 0, vk.DeviceSize(gIndexData.Sizeof()), 0, &indexDataPtr)
	n := vk.Memcopy(indexDataPtr, data)//gIndexData.Data())
	if n != int(size) {//gIndexData.Sizeof() {
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
