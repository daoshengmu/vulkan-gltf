# vulkan-gltf
Vulkan glTF model viewer 

https://github.com/KhronosGroup/glslang
glslangValidator.exe tri.vert -V -o tri-vert.spv
glslangValidator.exe tri.frag -V -o tri-frag.spv

https://github.com/jteeuwen/go-bindata
go-bindata shaders

Change package name of bindata.go

0. vertex/index buffer CmdBindVertexBuffers, CmdBindIndexBuffer
1. uniform VkDescriptorSetLayoutBinding -> more efficient, push constant
2. texture Vk.DescriptorSetLayoutBinding
