# vulkan-gltf
Vulkan glTF model viewer 

https://github.com/KhronosGroup/glslang
glslangValidator.exe tri.vert -V -o tri-vert.spv
glslangValidator.exe tri.frag -V -o tri-frag.spv

https://github.com/jteeuwen/go-bindata
go-bindata shaders

Change package name of bindata.go

1. vertex/index buffer CmdBindVertexBuffers, VkCmdBindIndexBuffer
2. uniform VkDescriptorSetLayoutBinding, VkCmdBindDescriptorSets
VkCreateDescriptorPool VkAllocateDescriptorSets/VkUpdateDescriptorSets -> more efficient, push constant
3. texture Vk.DescriptorSetLayoutBinding
