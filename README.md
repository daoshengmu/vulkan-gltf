# vulkan-samples
## Vulkan samples
- *drawTriangle*:
A simple example explains how to utlize a index/vertex buffer to render objects.
- *uniformBuffer*
A simple example describes how to use uniform buffer to send data to vertex shader to control object transformation.

## How to use
- We need glsl validator to compile our glsl programs. This is a new thing from Vulkan compared with OpenGL.
1. Download [glslang](https://github.com/KhronosGroup/glslang)
2. Compile your glsl code as below.
```
glslangValidator.exe tri.vert -V -o tri-vert.spv
glslangValidator.exe tri.frag -V -o tri-frag.spv
```
- Pack assets via [go-bindata](https://github.com/jteeuwen/go-bindata) for selected folder name as below.
```
go-bindata shaders
```
- Change package name of bindata.go from `main` to your current package name.
