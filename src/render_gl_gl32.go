//go:build !kinc && gl32

// This is almost identical to render_gl.go except it uses a VAO
// for GL 3.2 which is the minimum version that runs on modern
// macOS (Intel and ARM). Work adapted from assemblaj/fantasma

package main

import (
	"bytes"
	_ "embed" // Support for go:embed resources
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"strings"
	"unsafe"

	gl "github.com/go-gl/gl/v3.2-core/gl"
	glfw "github.com/go-gl/glfw/v3.3/glfw"
	mgl "github.com/go-gl/mathgl/mgl32"
	"golang.org/x/mobile/exp/f32"
)

const GL_SHADER_VER = 150 // OpenGL 3.2

var InternalFormatLUT = map[int32]uint32{
	8:  gl.RED,
	24: gl.RGB,
	32: gl.RGBA,
}

var BlendEquationLUT = map[BlendEquation]uint32{
	BlendAdd:             gl.FUNC_ADD,
	BlendReverseSubtract: gl.FUNC_REVERSE_SUBTRACT,
}

var BlendFunctionLUT = map[BlendFunc]uint32{
	BlendOne:              gl.ONE,
	BlendZero:             gl.ZERO,
	BlendSrcAlpha:         gl.SRC_ALPHA,
	BlendOneMinusSrcAlpha: gl.ONE_MINUS_SRC_ALPHA,
}

var PrimitiveModeLUT = map[PrimitiveMode]uint32{
	LINES:          gl.LINES,
	LINE_LOOP:      gl.LINE_LOOP,
	LINE_STRIP:     gl.LINE_STRIP,
	TRIANGLES:      gl.TRIANGLES,
	TRIANGLE_STRIP: gl.TRIANGLE_STRIP,
	TRIANGLE_FAN:   gl.TRIANGLE_FAN,
}

// ------------------------------------------------------------------
// Util
func glStr(s string) *uint8 {
	return gl.Str(s + "\x00")
}

// ------------------------------------------------------------------
// ShaderProgram

type ShaderProgram struct {
	// Program
	program uint32
	// Attributes
	a map[string]int32
	// Uniforms
	u map[string]int32
	// Texture units
	t map[string]int
}

func newShaderProgram(vert, frag, geo, id string, crashWhenFail bool, directives []string) (s *ShaderProgram, err error) {
	var vertObj, fragObj, geoObj, prog uint32
	if vertObj, err = compileShader(gl.VERTEX_SHADER, vert, directives); chkEX(err, "Shader compliation error on "+id+"\n", crashWhenFail) {
		return nil, err
	}
	if fragObj, err = compileShader(gl.FRAGMENT_SHADER, frag, directives); chkEX(err, "Shader compliation error on "+id+"\n", crashWhenFail) {
		return nil, err
	}
	if len(geo) > 0 {
		if geoObj, err = compileShader(gl.GEOMETRY_SHADER, geo, directives); chkEX(err, "Shader compliation error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
		if prog, err = linkProgram(vertObj, fragObj, geoObj); chkEX(err, "Link program error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
	} else {
		if prog, err = linkProgram(vertObj, fragObj); chkEX(err, "Link program error on "+id+"\n", crashWhenFail) {
			return nil, err
		}
	}
	s = &ShaderProgram{program: prog}
	s.a = make(map[string]int32)
	s.u = make(map[string]int32)
	s.t = make(map[string]int)
	return s, nil
}
func (s *ShaderProgram) RegisterAttributes(names ...string) {
	for _, name := range names {
		s.a[name] = gl.GetAttribLocation(s.program, glStr(name))
	}
}

func (s *ShaderProgram) RegisterUniforms(names ...string) {
	for _, name := range names {
		s.u[name] = gl.GetUniformLocation(s.program, glStr(name))
	}
}

func (s *ShaderProgram) RegisterTextures(names ...string) {
	for _, name := range names {
		s.u[name] = gl.GetUniformLocation(s.program, glStr(name))
		s.t[name] = len(s.t)
	}
}

func compileShaderBatch(shaderType uint32, version string, src string, directives []string) string {
	shader := version

	// setting preprocessor directives
	for _, directive := range directives {
		shader += directive + "\n"
	}

	if src == fragShader {
		shader += "vec4 get_tex(int index, sampler2D[MAX_TEXTURE_UNITS] textures, vec2 uv) {\n"
		for i := 0; i < batchRenderer.state.maxTextures-1; i++ {
			shader += fmt.Sprintf("\tif (index == %d) {\n", i)
			shader += fmt.Sprintf("\t\treturn texture(textures[%d], uv);\n", i)
			shader += "\t}\n"
		}
		shader += "\treturn texture(textures[0], uv);\n"
		shader += "}\n"
	}
	shader += src
	return shader
}

func compileShader(shaderType uint32, src string, directives []string) (shader uint32, err error) {
	shader = gl.CreateShader(shaderType)
	version := "#version 150\n"

	shdr := compileShaderBatch(shaderType, version, src, directives) + "\x00"

	s, _ := gl.Strs(shdr)
	var l int32 = int32(len(shdr) - 1)
	gl.ShaderSource(shader, 1, s, &l)
	gl.CompileShader(shader)
	var ok int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &ok)
	if ok == 0 {
		//var err error
		var size, l int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &size)
		if size > 0 {
			str := make([]byte, size+1)
			gl.GetShaderInfoLog(shader, size, &l, &str[0])
			err = Error(str[:l])
		} else {
			err = Error("Unknown shader compile error")
		}
		//chk(err)
		gl.DeleteShader(shader)
		//panic(Error("Shader compile error"))
		return 0, err
	}
	return shader, nil
}

func linkProgram(params ...uint32) (program uint32, err error) {
	program = gl.CreateProgram()
	for _, param := range params {
		gl.AttachShader(program, param)
	}
	if len(params) > 2 {
		// Geometry Shader Params
		gl.ProgramParameteri(program, gl.GEOMETRY_INPUT_TYPE, gl.TRIANGLES)
		gl.ProgramParameteri(program, gl.GEOMETRY_OUTPUT_TYPE, gl.TRIANGLE_STRIP)
		gl.ProgramParameteri(program, gl.GEOMETRY_VERTICES_OUT, 3*6)
	}
	gl.LinkProgram(program)
	// Mark shaders for deletion when the program is deleted
	for _, param := range params {
		gl.DeleteShader(param)
	}
	var ok int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &ok)
	if ok == 0 {
		//var err error
		var size, l int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &size)
		if size > 0 {
			str := make([]byte, size+1)
			gl.GetProgramInfoLog(program, size, &l, &str[0])
			err = Error(str[:l])
		} else {
			err = Error("Unknown link error")
		}
		//chk(err)
		gl.DeleteProgram(program)
		//panic(Error("Link error"))
		return 0, err
	}
	return program, nil
}

// ------------------------------------------------------------------
// Texture

type Texture struct {
	width  int32
	height int32
	depth  int32
	filter bool
	handle uint32
	layer  int32
}

// Generate a new texture name
func newTexture(width, height, depth int32, filter bool) (t *Texture) {
	var h uint32
	gl.ActiveTexture(gl.TEXTURE0)
	gl.GenTextures(1, &h)
	t = &Texture{width: width, height: height, depth: depth, filter: filter, handle: h}
	runtime.SetFinalizer(t, func(t *Texture) {
		sys.mainThreadTask <- func() {
			gl.DeleteTextures(1, &t.handle)
		}
	})
	return
}

func newDataTexture(width, height int32) (t *Texture) {
	var h uint32
	gl.ActiveTexture(gl.TEXTURE0)
	gl.GenTextures(1, &h)
	t = &Texture{width: width, height: height, depth: 32, filter: false, handle: h}
	runtime.SetFinalizer(t, func(t *Texture) {
		sys.mainThreadTask <- func() {
			gl.DeleteTextures(1, &t.handle)
		}
	})
	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	//gl.TexImage2D(gl.TEXTURE_2D, 0, 32, t.width, t.height, 0, 36, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return
}
func newHDRTexture(width, height int32) (t *Texture) {
	var h uint32
	gl.ActiveTexture(gl.TEXTURE0)
	gl.GenTextures(1, &h)
	t = &Texture{width: width, height: height, depth: 32, filter: false, handle: h}
	runtime.SetFinalizer(t, func(t *Texture) {
		sys.mainThreadTask <- func() {
			gl.DeleteTextures(1, &t.handle)
		}
	})
	gl.BindTexture(gl.TEXTURE_2D, t.handle)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.MIRRORED_REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.MIRRORED_REPEAT)
	return
}
func newCubeMapTexture(widthHeight int32, mipmap bool) (t *Texture) {
	var h uint32
	gl.ActiveTexture(gl.TEXTURE0)
	gl.GenTextures(1, &h)
	t = &Texture{width: widthHeight, height: widthHeight, depth: 32, filter: false, handle: h}
	runtime.SetFinalizer(t, func(t *Texture) {
		sys.mainThreadTask <- func() {
			gl.DeleteTextures(1, &t.handle)
		}
	})
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, t.handle)
	for i := 0; i < 6; i++ {
		gl.TexImage2D(uint32(gl.TEXTURE_CUBE_MAP_POSITIVE_X+i), 0, gl.RGB32F, widthHeight, widthHeight, 0, gl.RGB, gl.FLOAT, nil)
	}
	if mipmap {
		gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
		gl.GenerateMipmap(gl.TEXTURE_CUBE_MAP)
	} else {
		gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	}

	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return
}

// Bind a texture and upload texel data to it
func (t *Texture) SetData(data []byte) {
	var interp int32 = gl.NEAREST
	if t.filter {
		interp = gl.LINEAR
	}

	format := InternalFormatLUT[Max(t.depth, 8)]

	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	if data != nil {
		gl.TexImage2D(gl.TEXTURE_2D, 0, int32(format), t.width, t.height, 0, format, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
		//gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, t.width, t.height, format, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
	} else {
		gl.TexImage2D(gl.TEXTURE_2D, 0, int32(format), t.width, t.height, 0, format, gl.UNSIGNED_BYTE, nil)
		// gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, t.width, t.height, format, gl.UNSIGNED_BYTE, nil)
	}

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, interp)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, interp)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
}
func (t *Texture) SetDataG(data []byte, mag, min, ws, wt int32) {

	format := InternalFormatLUT[Max(t.depth, 8)]

	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, int32(format), t.width, t.height, 0, format, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, mag)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, min)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, ws)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wt)
}
func (t *Texture) SetPixelData(data []float32) {

	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, t.width, t.height, 0, gl.RGBA, gl.FLOAT, unsafe.Pointer(&data[0]))
	// gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, t.width, t.height, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
}

func (t *Texture) SetRGBPixelData(data []float32) {
	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB32F, t.width, t.height, 0, gl.RGB, gl.FLOAT, unsafe.Pointer(&data[0]))
}

func newTextureLayer(width, height, depth, layer int32, filter bool) (t *Texture) {
	t = &Texture{width: width, height: height, depth: depth, filter: filter, layer: layer}
	return
}

func (t *Texture) SetDataArray(layer int32, array uint32, data []byte) {
	format := InternalFormatLUT[Max(t.depth, 8)]
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, array)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	if array == batchRenderer.paletteTex {
		gl.TexSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, layer, 256, 1, 1, format, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
	}
}

// Return whether texture has a valid handle
func (t *Texture) IsValid() bool {
	return t.handle != 0
}

// ------------------------------------------------------------------
// Renderer

type Renderer struct {
	fbo         uint32
	fbo_texture uint32
	// Normal rendering
	rbo_depth uint32
	// MSAA rendering
	fbo_f         uint32
	fbo_f_texture *Texture
	// Shadow Map
	fbo_shadow              uint32
	fbo_shadow_texture      uint32
	fbo_shadow_cube_texture uint32
	fbo_env                 uint32
	// Post-processing shaders
	postVertBuffer   uint32
	postShaderSelect []*ShaderProgram
	// Shader and vertex data for primitive rendering
	spriteShader *ShaderProgram
	vertexBuffer uint32
	// Shader and index data for 3D model rendering
	shadowMapShader         *ShaderProgram
	modelShader             *ShaderProgram
	panoramaToCubeMapShader *ShaderProgram
	cubemapFilteringShader  *ShaderProgram
	stageVertexBuffer       uint32
	stageIndexBuffer        uint32
	vao                     uint32

	enableModel  bool
	enableShadow bool
}

//go:embed shaders/sprite_g32.vert.glsl
var vertShader string

//go:embed shaders/sprite_g32.frag.glsl
var fragShader string

//go:embed shaders/model.vert.glsl
var modelVertShader string

//go:embed shaders/model.frag.glsl
var modelFragShader string

//go:embed shaders/shadow.vert.glsl
var shadowVertShader string

//go:embed shaders/shadow.frag.glsl
var shadowFragShader string

//go:embed shaders/shadow.geo.glsl
var shadowGeoShader string

//go:embed shaders/ident.vert.glsl
var identVertShader string

//go:embed shaders/ident.frag.glsl
var identFragShader string

//go:embed shaders/panoramaToCubeMap.frag.glsl
var panoramaToCubeMapFragShader string

//go:embed shaders/cubemapFiltering.frag.glsl
var cubemapFilteringFragShader string

// init 3D model shader
func (r *Renderer) InitModelShader() error {
	var err error
	if r.enableShadow {
		r.modelShader, err = newShaderProgram(modelVertShader, "#define ENABLE_SHADOW\n"+modelFragShader, "", "Model Shader", false, nil)
	} else {
		r.modelShader, err = newShaderProgram(modelVertShader, modelFragShader, "", "Model Shader", false, nil)
	}
	if err != nil {
		return err
	}
	r.modelShader.RegisterAttributes("vertexId", "position", "uv", "normalIn", "tangentIn", "vertColor", "joints_0", "joints_1", "weights_0", "weights_1")
	r.modelShader.RegisterUniforms("model", "view", "projection", "farPlane", "normalMatrix", "unlit", "baseColorFactor", "add", "mult", "useTexture", "useNormalMap", "useMetallicRoughnessMap", "neg", "gray", "hue",
		"enableAlpha", "alphaThreshold", "numJoints", "morphTargetWeight", "morphTargetOffset", "morphTargetTextureDimension", "numTargets", "numVertices",
		"metallicRoughness", "ambientOcclusionStrength", "environmentIntensity", "mipCount",
		"cameraPosition", "environmentRotation",
		"lightMatrices[0]", "lightMatrices[1]", "lightMatrices[2]", "lightMatrices[3]",
		"lights[0].direction", "lights[0].range", "lights[0].color", "lights[0].intensity", "lights[0].position", "lights[0].innerConeCos", "lights[0].outerConeCos", "lights[0].type", "lights[0].shadowBias", "lights[0].shadowMapFar",
		"lights[1].direction", "lights[1].range", "lights[1].color", "lights[1].intensity", "lights[1].position", "lights[1].innerConeCos", "lights[1].outerConeCos", "lights[1].type", "lights[1].shadowBias", "lights[1].shadowMapFar",
		"lights[2].direction", "lights[2].range", "lights[2].color", "lights[2].intensity", "lights[2].position", "lights[2].innerConeCos", "lights[2].outerConeCos", "lights[2].type", "lights[2].shadowBias", "lights[2].shadowMapFar",
		"lights[3].direction", "lights[3].range", "lights[3].color", "lights[3].intensity", "lights[3].position", "lights[3].innerConeCos", "lights[3].outerConeCos", "lights[3].type", "lights[3].shadowBias", "lights[3].shadowMapFar",
	)
	r.modelShader.RegisterTextures("tex", "morphTargetValues", "jointMatrices", "normalMap", "metallicRoughnessMap", "ambientOcclusionMap", "lambertianEnvSampler", "GGXEnvSampler", "GGXLUT",
		"shadowMap", "shadowCubeMap")

	if r.enableShadow {
		r.shadowMapShader, err = newShaderProgram(shadowVertShader, shadowFragShader, shadowGeoShader, "Shadow Map Shader", false, nil)
		if err != nil {
			return err
		}
		r.shadowMapShader.RegisterAttributes("vertexId", "position", "vertColor", "uv", "joints_0", "joints_1", "weights_0", "weights_1")
		r.shadowMapShader.RegisterUniforms("model", "lightMatrices[0]", "lightMatrices[1]", "lightMatrices[2]", "lightMatrices[3]", "lightMatrices[4]", "lightMatrices[5]", "lightType", "lightPos", "farPlane", "numJoints", "morphTargetWeight", "morphTargetOffset", "morphTargetTextureDimension", "numTargets", "numVertices", "layerOffset", "enableAlpha", "alphaThreshold", "baseColorFactor", "useTexture")
		r.shadowMapShader.RegisterTextures("morphTargetValues", "jointMatrices", "tex")
	}
	r.panoramaToCubeMapShader, err = newShaderProgram(identVertShader, panoramaToCubeMapFragShader, "", "Panorama To Cubemap Shader", false, nil)
	if err != nil {
		return err
	}
	r.panoramaToCubeMapShader.RegisterAttributes("VertCoord")
	r.panoramaToCubeMapShader.RegisterUniforms("currentFace")
	r.panoramaToCubeMapShader.RegisterTextures("panorama")

	r.cubemapFilteringShader, err = newShaderProgram(identVertShader, cubemapFilteringFragShader, "", "Cubemap Filtering Shader", false, nil)
	if err != nil {
		return err
	}
	r.cubemapFilteringShader.RegisterAttributes("VertCoord")
	r.cubemapFilteringShader.RegisterUniforms("sampleCount", "distribution", "width", "currentFace", "roughness", "intensityScale", "isLUT")
	r.cubemapFilteringShader.RegisterTextures("cubeMap")
	return nil
}

func (r *Renderer) SupportsBatching() bool {
	return !(strings.Contains(gl.GoStr(gl.GetString(gl.VERSION)), "2.1"))
}

func GLDebugCallback(source uint32, gltype uint32, id uint32,
	severity uint32, length int32, message string, userParam unsafe.Pointer) {
	if gltype == gl.DEBUG_TYPE_ERROR {
		fmt.Println(fmt.Sprintf("GL CALLBACK: GL_ERROR type = 0x%x, severity = 0x%x, message = %s\n", gltype, severity, message))
	}
}

// Render initialization.
// Creates the default shaders, the framebuffer and enables MSAA.
func (r *Renderer) Init() {
	chk(gl.Init())
	sys.errLog.Printf("Using OpenGL %v (%v)", gl.GoStr(gl.GetString(gl.VERSION)), gl.GoStr(gl.GetString(gl.RENDERER)))

	if sys.debugGL {
		gl.Enable(gl.DEBUG_OUTPUT_SYNCHRONOUS)
		gl.DebugMessageCallback(GLDebugCallback, nil)
	}

	// Store current timestamp
	sys.prevTimestamp = glfw.GetTime()

	r.postShaderSelect = make([]*ShaderProgram, 1+len(sys.externalShaderList))

	// Data buffers for rendering
	postVertData := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)

	r.enableModel = sys.enableModel
	r.enableShadow = sys.enableModelShadow

	gl.GenVertexArrays(1, &r.vao)

	r.InitBatchRenderState()

	gl.GenBuffers(1, &r.postVertBuffer)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.postVertBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(postVertData), unsafe.Pointer(&postVertData[0]), gl.STATIC_DRAW)

	gl.GenBuffers(1, &r.vertexBuffer)

	r.InitBatchRendererBuffers()

	gl.GenBuffers(1, &r.stageVertexBuffer)
	gl.GenBuffers(1, &r.stageIndexBuffer)

	// Sprite shader
	r.InitBatchSpriteShader()

	r.InitUBOs()

	if r.enableModel {
		if err := r.InitModelShader(); err != nil {
			r.enableModel = false
		}
	}

	// Compile postprocessing shaders

	// Calculate total amount of shaders loaded.
	r.postShaderSelect = make([]*ShaderProgram, 1+len(sys.externalShaderList))

	// Ident shader (no postprocessing)
	r.postShaderSelect[0], _ = newShaderProgram(identVertShader, identFragShader, "", "Identity Postprocess", true, nil)
	r.postShaderSelect[0].RegisterAttributes("VertCoord", "TexCoord")
	r.postShaderSelect[0].RegisterUniforms("Texture", "TextureSize", "CurrentTime")

	// External Shaders
	for i := 0; i < len(sys.externalShaderList); i++ {
		r.postShaderSelect[1+i], _ = newShaderProgram(sys.externalShaders[0][i],
			sys.externalShaders[1][i], "", fmt.Sprintf("Postprocess Shader #%v", i+1), true, nil)
		r.postShaderSelect[1+i].RegisterAttributes("VertCoord", "TexCoord")
		loc := r.postShaderSelect[0].a["TexCoord"]
		gl.VertexAttribPointer(uint32(loc), 3, gl.FLOAT, false, 5*4, gl.PtrOffset(2*4))
		gl.EnableVertexAttribArray(uint32(loc))
		r.postShaderSelect[1+i].RegisterUniforms("Texture", "TextureSize", "CurrentTime")
	}

	if sys.multisampleAntialiasing > 0 {
		gl.Enable(gl.MULTISAMPLE)
	}

	gl.ActiveTexture(gl.TEXTURE0)
	gl.GenTextures(1, &r.fbo_texture)

	if sys.multisampleAntialiasing > 0 {
		gl.BindTexture(gl.TEXTURE_2D_MULTISAMPLE, r.fbo_texture)
	} else {
		gl.BindTexture(gl.TEXTURE_2D, r.fbo_texture)
	}

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	if sys.multisampleAntialiasing > 0 {
		gl.TexImage2DMultisample(gl.TEXTURE_2D_MULTISAMPLE, sys.multisampleAntialiasing, gl.RGBA, sys.scrrect[2], sys.scrrect[3], true)

	} else {
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, sys.scrrect[2], sys.scrrect[3], 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	}

	gl.BindTexture(gl.TEXTURE_2D, 0)

	r.InitPaletteTextureArray()

	//r.rbo_depth = gl.CreateRenderbuffer()
	gl.GenRenderbuffers(1, &r.rbo_depth)

	gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo_depth)
	if sys.multisampleAntialiasing > 0 {
		//gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, int(sys.scrrect[2]), int(sys.scrrect[3]))
		gl.RenderbufferStorageMultisample(gl.RENDERBUFFER, sys.multisampleAntialiasing, gl.DEPTH_COMPONENT16, sys.scrrect[2], sys.scrrect[3])
	} else {
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, sys.scrrect[2], sys.scrrect[3])
	}
	gl.BindRenderbuffer(gl.RENDERBUFFER, 0)
	if sys.multisampleAntialiasing > 0 {
		r.fbo_f_texture = newTexture(sys.scrrect[2], sys.scrrect[3], 32, false)
		r.fbo_f_texture.SetData(nil)
	} else {
		//r.rbo_depth = gl.CreateRenderbuffer()
		//gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo_depth)
		//gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, int(sys.scrrect[2]), int(sys.scrrect[3]))
		//gl.BindRenderbuffer(gl.RENDERBUFFER, gl.NoRenderbuffer)
	}

	gl.GenFramebuffers(1, &r.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)

	if sys.multisampleAntialiasing > 0 {
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D_MULTISAMPLE, r.fbo_texture, 0)
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, r.rbo_depth)
		if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
			sys.errLog.Printf("framebuffer create failed: 0x%x", status)
			fmt.Printf("framebuffer create failed: 0x%x \n", status)
		}
		gl.GenFramebuffers(1, &r.fbo_f)
		gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_f)
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.fbo_f_texture.handle, 0)
	} else {
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.fbo_texture, 0)
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, r.rbo_depth)
	}

	if r.enableModel {
		if r.enableShadow {
			gl.GenFramebuffers(1, &r.fbo_shadow)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.GenTextures(1, &r.fbo_shadow_texture)
			gl.GenTextures(1, &r.fbo_shadow_cube_texture)

			gl.BindTexture(gl.TEXTURE_2D_ARRAY, r.fbo_shadow_texture)
			gl.TexStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.DEPTH_COMPONENT24, 1024, 1024, 4)
			gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
			gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

			gl.BindTexture(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, r.fbo_shadow_cube_texture)
			gl.TexStorage3D(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, 1, gl.DEPTH_COMPONENT24, 1024, 1024, 4*6)
			gl.TexParameteri(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
			gl.TexParameteri(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
			gl.TexParameteri(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

			gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_shadow)
			gl.DrawBuffer(gl.NONE)
			gl.ReadBuffer(gl.NONE)
			if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
				sys.errLog.Printf("framebuffer create failed: 0x%x", status)
			}
		}
		gl.GenFramebuffers(1, &r.fbo_env)
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.BindVertexArray(r.vao)
}

func (r *Renderer) Close() {
}

func (r *Renderer) BeginFrame(clearColor bool) {
	sys.absTickCountF++
	gl.BindVertexArray(r.vao)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	gl.Viewport(0, 0, sys.scrrect[2], sys.scrrect[3])
	if clearColor {
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	} else {
		gl.Clear(gl.DEPTH_BUFFER_BIT)
	}
}

func (r *Renderer) BlendReset() {
	gl.BlendEquation(BlendEquationLUT[BlendAdd])
	gl.BlendFunc(BlendFunctionLUT[BlendSrcAlpha], BlendFunctionLUT[BlendOneMinusSrcAlpha])
}
func (r *Renderer) EndFrame() {
	gl.BindVertexArray(r.vao)

	if sys.multisampleAntialiasing > 0 {
		gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, r.fbo_f)
		gl.BindFramebuffer(gl.READ_FRAMEBUFFER, r.fbo)
		gl.BlitFramebuffer(0, 0, sys.scrrect[2], sys.scrrect[3], 0, 0, sys.scrrect[2], sys.scrrect[3], gl.COLOR_BUFFER_BIT, gl.LINEAR)
	}

	x, y, resizedWidth, resizedHeight := sys.window.GetScaledViewportSize()
	postShader := r.postShaderSelect[sys.postProcessingShader]

	var scaleMode int32 // GL enum
	if sys.windowScaleMode {
		scaleMode = gl.LINEAR
	} else {
		scaleMode = gl.NEAREST
	}

	gl.Viewport(x, y, int32(resizedWidth), int32(resizedHeight))
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	gl.UseProgram(postShader.program)
	gl.Disable(gl.BLEND)

	gl.ActiveTexture(gl.TEXTURE0)
	if sys.multisampleAntialiasing > 0 {
		gl.BindTexture(gl.TEXTURE_2D, r.fbo_f_texture.handle)
	} else {
		gl.BindTexture(gl.TEXTURE_2D, r.fbo_texture)
	}

	// set post-processing parameters
	gl.Uniform1i(postShader.u["Texture"], 0)
	gl.Uniform2f(postShader.u["TextureSize"], float32(resizedWidth), float32(resizedHeight))
	gl.Uniform1f(postShader.u["CurrentTime"], float32(glfw.GetTime()))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, scaleMode)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, scaleMode)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.postVertBuffer)

	loc := postShader.a["VertCoord"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointer(uint32(loc), 2, gl.FLOAT, false, 0, nil)

	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.DisableVertexAttribArray(uint32(loc))

	// r.SwitchBuffer()
	gl.BindVertexArray(0)
}

func (r *Renderer) SetPipeline(eq BlendEquation, src, dst BlendFunc) {
	gl.BindVertexArray(r.vao)
	gl.UseProgram(r.spriteShader.program)

	gl.BlendEquation(BlendEquationLUT[eq])
	gl.BlendFunc(BlendFunctionLUT[src], BlendFunctionLUT[dst])
	gl.Enable(gl.BLEND)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// Must bind buffer before enabling attributes
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
	loc := r.spriteShader.a["position"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 20, 0)
	loc = r.spriteShader.a["uv"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 20, 8)
	loc = r.spriteShader.a["index"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 1, gl.FLOAT, false, 20, 16)

}

func (r *Renderer) ReleasePipeline() {
	loc := r.spriteShader.a["position"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.spriteShader.a["uv"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.spriteShader.a["index"]
	gl.DisableVertexAttribArray(uint32(loc))
	gl.Disable(gl.BLEND)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
}

func (r *Renderer) prepareShadowMapPipeline() {
	gl.UseProgram(r.shadowMapShader.program)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_shadow)
	gl.Viewport(0, 0, 1024, 1024)
	gl.Enable(gl.TEXTURE_2D)
	gl.Disable(gl.BLEND)
	gl.Enable(gl.DEPTH_TEST)
	//gl.DepthFunc(gl.LESS)
	//gl.DepthMask(true)

	gl.BlendEquation(gl.FUNC_ADD)
	gl.BlendFunc(gl.ONE, gl.ZERO)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.stageVertexBuffer)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.stageIndexBuffer)

	gl.FramebufferTexture(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, r.fbo_shadow_texture, 0)
	gl.Clear(gl.DEPTH_BUFFER_BIT)
	gl.FramebufferTexture(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, r.fbo_shadow_cube_texture, 0)
	gl.Clear(gl.DEPTH_BUFFER_BIT)
}
func (r *Renderer) setShadowMapPipeline(doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
	if invertFrontFace {
		gl.FrontFace(gl.CW)
	} else {
		gl.FrontFace(gl.CCW)
	}
	if !doubleSided {
		gl.Enable(gl.CULL_FACE)
		gl.CullFace(gl.BACK)
	} else {
		gl.Disable(gl.CULL_FACE)
	}

	loc := r.shadowMapShader.a["vertexId"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 1, gl.INT, false, 0, uintptr(vertAttrOffset))
	offset := vertAttrOffset + 4*numVertices

	loc = r.shadowMapShader.a["position"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 3, gl.FLOAT, false, 0, uintptr(offset))
	offset += 12 * numVertices
	if useUV {
		loc = r.shadowMapShader.a["uv"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 0, uintptr(offset))
		offset += 8 * numVertices
	} else {
		loc = r.shadowMapShader.a["uv"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib2f(uint32(loc), 0, 0)
	}
	if useNormal {
		offset += 12 * numVertices
	}
	if useTangent {
		offset += 16 * numVertices
	}
	if useVertColor {
		loc = r.shadowMapShader.a["vertColor"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
	} else {
		loc = r.shadowMapShader.a["vertColor"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib4f(uint32(loc), 1, 1, 1, 1)
	}
	if useJoint0 {
		loc = r.shadowMapShader.a["joints_0"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
		loc = r.shadowMapShader.a["weights_0"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
		if useJoint1 {
			loc = r.shadowMapShader.a["joints_1"]
			gl.EnableVertexAttribArray(uint32(loc))
			gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
			offset += 16 * numVertices
			loc = r.shadowMapShader.a["weights_1"]
			gl.EnableVertexAttribArray(uint32(loc))
			gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
			offset += 16 * numVertices
		} else {
			loc = r.shadowMapShader.a["joints_1"]
			gl.DisableVertexAttribArray(uint32(loc))
			gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
			loc = r.shadowMapShader.a["weights_1"]
			gl.DisableVertexAttribArray(uint32(loc))
			gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		}
	} else {
		loc = r.shadowMapShader.a["joints_0"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.shadowMapShader.a["weights_0"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.shadowMapShader.a["joints_1"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.shadowMapShader.a["weights_1"]
		gl.DisableVertexAttribArray(uint32(loc))
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
	}
}

func (r *Renderer) ReleaseShadowPipeline() {
	loc := r.modelShader.a["vertexId"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["position"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["uv"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["vertColor"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["joints_0"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["weights_0"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["joints_1"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["weights_1"]
	gl.DisableVertexAttribArray(uint32(loc))
	//gl.Disable(gl.TEXTURE_2D)
	gl.DepthMask(true)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.BLEND)
}
func (r *Renderer) prepareModelPipeline(env *Environment) {
	gl.UseProgram(r.modelShader.program)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	gl.Viewport(0, 0, sys.scrrect[2], sys.scrrect[3])
	gl.Clear(gl.DEPTH_BUFFER_BIT)
	gl.Enable(gl.TEXTURE_2D)
	gl.Enable(gl.TEXTURE_CUBE_MAP)
	gl.Enable(gl.BLEND)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.stageVertexBuffer)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.stageIndexBuffer)
	if r.enableShadow {
		loc, unit := r.modelShader.u["shadowMap"], r.modelShader.t["shadowMap"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_2D_ARRAY, r.fbo_shadow_texture)
		gl.Uniform1i(loc, int32(unit))

		loc, unit = r.modelShader.u["shadowCubeMap"], r.modelShader.t["shadowCubeMap"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_CUBE_MAP_ARRAY_ARB, r.fbo_shadow_cube_texture)
		gl.Uniform1i(loc, int32(unit))
	}
	if env != nil {
		loc, unit := r.modelShader.u["lambertianEnvSampler"], r.modelShader.t["lambertianEnvSampler"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, env.lambertianTexture.tex.handle)
		gl.Uniform1i(loc, int32(unit))
		loc, unit = r.modelShader.u["GGXEnvSampler"], r.modelShader.t["GGXEnvSampler"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, env.GGXTexture.tex.handle)
		gl.Uniform1i(loc, int32(unit))
		loc, unit = r.modelShader.u["GGXLUT"], r.modelShader.t["GGXLUT"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_2D, env.GGXLUT.tex.handle)
		gl.Uniform1i(loc, int32(unit))

		loc = r.modelShader.u["environmentIntensity"]
		gl.Uniform1f(loc, env.environmentIntensity)
		loc = r.modelShader.u["mipCount"]
		gl.Uniform1i(loc, env.mipmapLevels)
		loc = r.modelShader.u["environmentRotation"]
		rotationMatrix := mgl.Rotate3DX(math.Pi).Mul3(mgl.Rotate3DY(0.5 * math.Pi))
		rotationM := rotationMatrix[:]
		gl.UniformMatrix3fv(loc, 1, false, &rotationM[0])

	} else {
		loc, unit := r.modelShader.u["lambertianEnvSampler"], r.modelShader.t["lambertianEnvSampler"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
		gl.Uniform1i(loc, int32(unit))
		loc, unit = r.modelShader.u["GGXEnvSampler"], r.modelShader.t["GGXEnvSampler"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
		gl.Uniform1i(loc, int32(unit))
		loc, unit = r.modelShader.u["GGXLUT"], r.modelShader.t["GGXLUT"]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_2D, 0)
		gl.Uniform1i(loc, int32(unit))
		loc = r.modelShader.u["environmentIntensity"]
		gl.Uniform1f(loc, 0)
	}
}
func (r *Renderer) SetModelPipeline(eq BlendEquation, src, dst BlendFunc, depthTest, depthMask, doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
	if depthTest {
		gl.Enable(gl.DEPTH_TEST)
		gl.DepthFunc(gl.LESS)
	} else {
		gl.Disable(gl.DEPTH_TEST)
	}
	gl.DepthMask(depthMask)
	if invertFrontFace {
		gl.FrontFace(gl.CW)
	} else {
		gl.FrontFace(gl.CCW)
	}
	if !doubleSided {
		gl.Enable(gl.CULL_FACE)
		gl.CullFace(gl.BACK)
	} else {
		gl.Disable(gl.CULL_FACE)
	}

	gl.BlendEquation(BlendEquationLUT[eq])
	gl.BlendFunc(BlendFunctionLUT[src], BlendFunctionLUT[dst])

	loc := r.modelShader.a["vertexId"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 1, gl.INT, false, 0, uintptr(vertAttrOffset))
	offset := vertAttrOffset + 4*numVertices

	loc = r.modelShader.a["position"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 3, gl.FLOAT, false, 0, uintptr(offset))
	offset += 12 * numVertices
	if useUV {
		loc = r.modelShader.a["uv"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 0, uintptr(offset))
		offset += 8 * numVertices
	} else {
		loc = r.modelShader.a["uv"]
		gl.VertexAttrib2f(uint32(loc), 0, 0)
	}
	if useNormal {
		loc = r.modelShader.a["normalIn"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 3, gl.FLOAT, false, 0, uintptr(offset))
		offset += 12 * numVertices
	} else {
		loc = r.modelShader.a["normalIn"]
		gl.VertexAttrib3f(uint32(loc), 0, 0, 0)
	}
	if useTangent {
		loc = r.modelShader.a["tangentIn"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
	} else {
		loc = r.modelShader.a["tangentIn"]
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
	}
	if useVertColor {
		loc = r.modelShader.a["vertColor"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
	} else {
		loc = r.modelShader.a["vertColor"]
		gl.VertexAttrib4f(uint32(loc), 1, 1, 1, 1)
	}
	if useJoint0 {
		loc = r.modelShader.a["joints_0"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
		loc = r.modelShader.a["weights_0"]
		gl.EnableVertexAttribArray(uint32(loc))
		gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
		offset += 16 * numVertices
		if useJoint1 {
			loc = r.modelShader.a["joints_1"]
			gl.EnableVertexAttribArray(uint32(loc))
			gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
			offset += 16 * numVertices
			loc = r.modelShader.a["weights_1"]
			gl.EnableVertexAttribArray(uint32(loc))
			gl.VertexAttribPointerWithOffset(uint32(loc), 4, gl.FLOAT, false, 0, uintptr(offset))
			offset += 16 * numVertices
		} else {
			loc = r.modelShader.a["joints_1"]
			gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
			loc = r.modelShader.a["weights_1"]
			gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		}
	} else {
		loc = r.modelShader.a["joints_0"]
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.modelShader.a["weights_0"]
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.modelShader.a["joints_1"]
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
		loc = r.modelShader.a["weights_1"]
		gl.VertexAttrib4f(uint32(loc), 0, 0, 0, 0)
	}
}
func (r *Renderer) ReleaseModelPipeline() {
	loc := r.modelShader.a["vertexId"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["position"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["uv"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["vertColor"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["joints_0"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["weights_0"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["joints_1"]
	gl.DisableVertexAttribArray(uint32(loc))
	loc = r.modelShader.a["weights_1"]
	gl.DisableVertexAttribArray(uint32(loc))
	//gl.Disable(gl.TEXTURE_2D)
	gl.DepthMask(true)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
}

func (r *Renderer) ReadPixels(data []uint8, width, height int) {
	// we defer the EndFrame(), SwapBuffers(), and BeginFrame() calls that were previously below now to
	// a single spot in order to prevent the blank screenshot bug on single digit FPS
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&data[0]))
}

func (r *Renderer) Scissor(x, y, width, height int32) {
	gl.Enable(gl.SCISSOR_TEST)
	gl.Scissor(x, sys.scrrect[3]-(y+height), width, height)
}

func (r *Renderer) DisableScissor() {
	gl.Disable(gl.SCISSOR_TEST)
}

func (r *Renderer) SetUniformI(name string, val int) {
	loc := r.spriteShader.u[name]
	gl.Uniform1i(loc, int32(val))
}

func (r *Renderer) SetUniformF(name string, values ...float32) {
	loc := r.spriteShader.u[name]
	switch len(values) {
	case 1:
		gl.Uniform1f(loc, values[0])
	case 2:
		gl.Uniform2f(loc, values[0], values[1])
	case 3:
		gl.Uniform3f(loc, values[0], values[1], values[2])
	case 4:
		gl.Uniform4f(loc, values[0], values[1], values[2], values[3])
	}
}

func (r *Renderer) SetUniformFv(name string, values []float32) {
	loc := r.spriteShader.u[name]
	switch len(values) {
	case 2:
		gl.Uniform2fv(loc, 1, &values[0])
	case 3:
		gl.Uniform3fv(loc, 1, &values[0])
	case 4:
		gl.Uniform4fv(loc, 1, &values[0])
	}
}

func (r *Renderer) SetUniformMatrix(name string, value []float32) {
	loc := r.spriteShader.u[name]
	gl.UniformMatrix4fv(loc, 1, false, &value[0])
}

func (r *Renderer) SetTexture(name string, t *Texture) {
	loc, unit := r.spriteShader.u[name], r.spriteShader.t[name]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.Uniform1i(loc, int32(unit))
}

func (r *Renderer) SetModelUniformI(name string, val int) {
	loc := r.modelShader.u[name]
	gl.Uniform1i(loc, int32(val))
}

func (r *Renderer) SetModelUniformF(name string, values ...float32) {
	loc := r.modelShader.u[name]
	switch len(values) {
	case 1:
		gl.Uniform1f(loc, values[0])
	case 2:
		gl.Uniform2f(loc, values[0], values[1])
	case 3:
		gl.Uniform3f(loc, values[0], values[1], values[2])
	case 4:
		gl.Uniform4f(loc, values[0], values[1], values[2], values[3])
	}
}
func (r *Renderer) SetModelUniformFv(name string, values []float32) {
	loc := r.modelShader.u[name]
	switch len(values) {
	case 2:
		gl.Uniform2fv(loc, 1, &values[0])
	case 3:
		gl.Uniform3fv(loc, 1, &values[0])
	case 4:
		gl.Uniform4fv(loc, 1, &values[0])
	case 8:
		gl.Uniform4fv(loc, 2, &values[0])
	}
}
func (r *Renderer) SetModelUniformMatrix(name string, value []float32) {
	loc := r.modelShader.u[name]
	gl.UniformMatrix4fv(loc, 1, false, &value[0])
}

func (r *Renderer) SetModelTexture(name string, t *Texture) {
	loc, unit := r.modelShader.u[name], r.modelShader.t[name]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.Uniform1i(loc, int32(unit))
}

func (r *Renderer) SetShadowMapUniformI(name string, val int) {
	loc := r.shadowMapShader.u[name]
	gl.Uniform1i(loc, int32(val))
}

func (r *Renderer) SetShadowMapUniformF(name string, values ...float32) {
	loc := r.shadowMapShader.u[name]
	switch len(values) {
	case 1:
		gl.Uniform1f(loc, values[0])
	case 2:
		gl.Uniform2f(loc, values[0], values[1])
	case 3:
		gl.Uniform3f(loc, values[0], values[1], values[2])
	case 4:
		gl.Uniform4f(loc, values[0], values[1], values[2], values[3])
	}
}
func (r *Renderer) SetShadowMapUniformFv(name string, values []float32) {
	loc := r.shadowMapShader.u[name]
	switch len(values) {
	case 2:
		gl.Uniform2fv(loc, 1, &values[0])
	case 3:
		gl.Uniform3fv(loc, 1, &values[0])
	case 4:
		gl.Uniform4fv(loc, 1, &values[0])
	case 8:
		gl.Uniform4fv(loc, 2, &values[0])
	}
}
func (r *Renderer) SetShadowMapUniformMatrix(name string, value []float32) {
	loc := r.shadowMapShader.u[name]
	gl.UniformMatrix4fv(loc, 1, false, &value[0])
}

func (r *Renderer) SetShadowMapTexture(name string, t *Texture) {
	loc, unit := r.shadowMapShader.u[name], r.shadowMapShader.t[name]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_2D, t.handle)
	gl.Uniform1i(loc, int32(unit))
}

func (r *Renderer) SetShadowFrameTexture(i uint32) {
	gl.FramebufferTextureLayer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, r.fbo_shadow_texture, 0, int32(i))
	//gl.Clear(gl.DEPTH_BUFFER_BIT)
}

func (r *Renderer) SetShadowFrameCubeTexture(i uint32) {
	gl.FramebufferTexture(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, r.fbo_shadow_cube_texture, 0)
	//gl.Clear(gl.DEPTH_BUFFER_BIT)
}

func (r *Renderer) SetStageVertexData(values []byte) {
	gl.BindBuffer(gl.ARRAY_BUFFER, r.stageVertexBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(values), unsafe.Pointer(&values[0]), gl.STATIC_DRAW)
}
func (r *Renderer) SetStageIndexData(values ...uint32) {
	data := new(bytes.Buffer)
	binary.Write(data, binary.LittleEndian, values)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.stageIndexBuffer)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(values)*4, unsafe.Pointer(&data.Bytes()[0]), gl.STATIC_DRAW)
}

func (r *Renderer) RenderQuad() {
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
}
func (r *Renderer) RenderElements(mode PrimitiveMode, count, offset int) {
	gl.DrawElementsWithOffset(PrimitiveModeLUT[mode], int32(count), gl.UNSIGNED_INT, uintptr(offset))
}

func (r *Renderer) RenderCubeMap(envTexture *Texture, cubeTexture *Texture, textureSize int32) {
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_env)
	gl.Viewport(0, 0, textureSize, textureSize)
	gl.UseProgram(r.panoramaToCubeMapShader.program)
	loc := r.panoramaToCubeMapShader.a["VertCoord"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 0, 0)
	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(data), unsafe.Pointer(&data[0]), gl.STATIC_DRAW)
	loc, unit := r.panoramaToCubeMapShader.u["panorama"], r.panoramaToCubeMapShader.t["panorama"]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_2D, envTexture.handle)
	gl.Uniform1i(loc, int32(unit))
	for i := 0; i < 6; i++ {
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, uint32(gl.TEXTURE_CUBE_MAP_POSITIVE_X+i), cubeTexture.handle, 0)

		gl.Clear(gl.COLOR_BUFFER_BIT)
		loc := r.panoramaToCubeMapShader.u["currentFace"]
		gl.Uniform1i(loc, int32(i))

		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubeTexture.handle)
	gl.GenerateMipmap(gl.TEXTURE_CUBE_MAP)
}
func (r *Renderer) RenderFilteredCubeMap(distribution int32, cubeTexture *Texture, filteredTexture *Texture, textureSize, mipmapLevel, sampleCount int32, roughness float32) {
	currentTextureSize := textureSize >> mipmapLevel
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_env)
	gl.Viewport(0, 0, currentTextureSize, currentTextureSize)
	gl.UseProgram(r.cubemapFilteringShader.program)
	loc := r.cubemapFilteringShader.a["VertCoord"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 0, 0)
	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(data), unsafe.Pointer(&data[0]), gl.STATIC_DRAW)
	loc, unit := r.cubemapFilteringShader.u["cubeMap"], r.cubemapFilteringShader.t["cubeMap"]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubeTexture.handle)
	gl.Uniform1i(loc, int32(unit))
	loc = r.cubemapFilteringShader.u["sampleCount"]
	gl.Uniform1i(loc, sampleCount)
	loc = r.cubemapFilteringShader.u["distribution"]
	gl.Uniform1i(loc, distribution)
	loc = r.cubemapFilteringShader.u["width"]
	gl.Uniform1i(loc, currentTextureSize)
	loc = r.cubemapFilteringShader.u["roughness"]
	gl.Uniform1f(loc, roughness)
	loc = r.cubemapFilteringShader.u["intensityScale"]
	gl.Uniform1f(loc, 1)
	loc = r.cubemapFilteringShader.u["isLUT"]
	gl.Uniform1i(loc, 0)
	for i := 0; i < 6; i++ {
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, uint32(gl.TEXTURE_CUBE_MAP_POSITIVE_X+i), filteredTexture.handle, mipmapLevel)

		gl.Clear(gl.COLOR_BUFFER_BIT)
		loc := r.cubemapFilteringShader.u["currentFace"]
		gl.Uniform1i(loc, int32(i))

		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
}
func (r *Renderer) RenderLUT(distribution int32, cubeTexture *Texture, lutTexture *Texture, textureSize, sampleCount int32) {
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo_env)
	gl.Viewport(0, 0, textureSize, textureSize)
	gl.UseProgram(r.cubemapFilteringShader.program)
	loc := r.cubemapFilteringShader.a["VertCoord"]
	gl.EnableVertexAttribArray(uint32(loc))
	gl.VertexAttribPointerWithOffset(uint32(loc), 2, gl.FLOAT, false, 0, 0)
	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(data), unsafe.Pointer(&data[0]), gl.STATIC_DRAW)
	loc, unit := r.cubemapFilteringShader.u["cubeMap"], r.cubemapFilteringShader.t["cubeMap"]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubeTexture.handle)
	gl.Uniform1i(loc, int32(unit))
	loc = r.cubemapFilteringShader.u["sampleCount"]
	gl.Uniform1i(loc, sampleCount)
	loc = r.cubemapFilteringShader.u["distribution"]
	gl.Uniform1i(loc, distribution)
	loc = r.cubemapFilteringShader.u["width"]
	gl.Uniform1i(loc, textureSize)
	loc = r.cubemapFilteringShader.u["roughness"]
	gl.Uniform1f(loc, 0)
	loc = r.cubemapFilteringShader.u["intensityScale"]
	gl.Uniform1f(loc, 1)
	loc = r.cubemapFilteringShader.u["currentFace"]
	gl.Uniform1i(loc, 0)
	loc = r.cubemapFilteringShader.u["isLUT"]
	gl.Uniform1i(loc, 1)

	gl.BindTexture(gl.TEXTURE_2D, lutTexture.handle)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, lutTexture.width, lutTexture.height, 0, gl.RGBA, gl.FLOAT, nil)

	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, lutTexture.handle, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
}

func (r *Renderer) InitBatchRenderState() {
	batchRenderer.setInitialUniforms = true
	// Check the maximum number of texture units
	gl.GetIntegerv(gl.MAX_TEXTURE_IMAGE_UNITS, &batchRenderer.maxTextureUnits)
	batchRenderer.state.maxTextures = int(batchRenderer.maxTextureUnits)
	batchRenderer.vertexBufferCache = make(map[uint64]uint32)

}

func (r *Renderer) InitBatchRendererBuffers() {
	gl.GenBuffers(1, &batchRenderer.vertexBuffer2)
	batchRenderer.curVertexBuffer = r.vertexBuffer

	gl.GenBuffers(1, &batchRenderer.fragUbo)
	gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.fragUbo)
	gl.BufferData(gl.UNIFORM_BUFFER, int(128*96), nil, gl.STATIC_DRAW)

	gl.GenBuffers(1, &batchRenderer.vertUbo)
	gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.vertUbo)
	gl.BufferData(gl.UNIFORM_BUFFER, int(32*unsafe.Sizeof(VertexUniforms{})), nil, gl.STATIC_DRAW)

	//gl.GenBuffers(1, &batchRenderer.indexUbo)
	//gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.indexUbo)
	//gl.BufferData(gl.UNIFORM_BUFFER, int(4096*4), nil, gl.STATIC_DRAW)

}

func (r *Renderer) SetVertexData(values ...float32) {
	data := batchF32Encode(values)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
	gl.BufferData(gl.ARRAY_BUFFER, len(data), unsafe.Pointer(&data[0]), gl.STATIC_DRAW)
}

func (r *Renderer) SetTextureArrayWithHandle(name string, handle uint32) {
	loc, _ := r.spriteShader.u[name], r.spriteShader.t[name]
	gl.ActiveTexture((uint32(gl.TEXTURE16)))
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, handle)
	gl.Uniform1i(loc, 16)
}

func (r *Renderer) SetTextureWithHandle(name string, handle uint32) {
	loc, unit := r.spriteShader.u[name], r.spriteShader.t[name]
	gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
	gl.BindTexture(gl.TEXTURE_2D, handle)
	gl.Uniform1i(loc, int32(unit))
}

func (r *Renderer) SetTextures(name string, handles []uint32) []int32 {
	loc, unit := r.spriteShader.u[name], r.spriteShader.t[name]
	indices := make([]int32, len(handles))
	for i := 0; i < len(handles); i++ {
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit + i)))
		gl.BindTexture(gl.TEXTURE_2D, handles[i])
		indices[i] = int32(unit + i)
		gl.Uniform1i(loc, int32(unit+i))
	}
	return indices
}

func (r *Renderer) UnbindTextures(name []string) {
	for i := 0; i < len(name); i++ {
		_, unit := r.spriteShader.u[name[i]], r.spriteShader.t[name[i]]
		gl.ActiveTexture((uint32(gl.TEXTURE0 + unit)))
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}
}

func (r *Renderer) RenderQuadAtIndex(vertex int32) {
	gl.DrawArrays(gl.TRIANGLE_STRIP, vertex, 4)
}

func (r *Renderer) RenderQuadBatch(total int32) {
	gl.DrawArrays(gl.TRIANGLES, 0, total)
}

func (r *Renderer) RenderQuadBatchAtIndex(first int32, count int32) {
	gl.DrawArrays(gl.TRIANGLES, first, count)
}

func (r *Renderer) SwitchBuffer() {
	if batchRenderer.curVertexBuffer == gfx.vertexBuffer {
		batchRenderer.curVertexBuffer = batchRenderer.vertexBuffer2
	} else {
		batchRenderer.curVertexBuffer = gfx.vertexBuffer
	}
}

func (r *Renderer) UploadFragmentUBO(uniforms []FragmentUniforms) {
	if len(uniforms) == 0 {
		return
	}

	buf := make([]byte, 0, 96*len(uniforms))
	for i := 0; i < len(uniforms); i++ {
		buf = append(buf, uniforms[i].ToBytes()...)
	}

	gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.fragUbo)
	gl.BufferSubData(gl.UNIFORM_BUFFER, 0, len(buf), unsafe.Pointer(&buf[0]))
}

func (r *Renderer) UploadVertexUBO(uniforms []VertexUniforms) {
	buf := make([]byte, 0, int(unsafe.Sizeof(VertexUniforms{}))*len(uniforms))
	for i := 0; i < len(uniforms); i++ {
		buf = append(buf, uniforms[i].ToBytes()...)
	}

	gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.vertUbo)
	gl.BufferSubData(gl.UNIFORM_BUFFER, 0, len(buf), unsafe.Pointer(&buf[0]))
}

func (r *Renderer) UploadIndexUniformUBO(uniforms []IndexUniforms) {
	buf := make([]byte, 0, int(4)*len(uniforms))
	for i := 0; i < len(uniforms); i++ {
		buf = append(buf, uniforms[i].ToBytes()...)
	}

	gl.BindBuffer(gl.UNIFORM_BUFFER, batchRenderer.indexUbo)
	gl.BufferSubData(gl.UNIFORM_BUFFER, 0, len(buf), unsafe.Pointer(&buf[0]))
}

func (r *Renderer) BindUBOs() {
	gl.BindBufferBase(gl.UNIFORM_BUFFER, 0, batchRenderer.fragUbo)
	gl.BindBufferBase(gl.UNIFORM_BUFFER, 1, batchRenderer.vertUbo)
	// gl.BindBufferBase(gl.UNIFORM_BUFFER, 2, batchRenderer.indexUbo)
}

func (r *Renderer) InitPaletteTextureArray() {
	gl.ActiveTexture(gl.TEXTURE16)
	gl.GenTextures(1, &batchRenderer.paletteTex)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, batchRenderer.paletteTex)
	gl.TexImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.RGBA, 256, 1, 512, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, 0)

}

func (r *Renderer) InitUBOs() {
	var bindingPoint uint32 = 0
	blockIndex := gl.GetUniformBlockIndex(r.spriteShader.program, glStr("FragmentUniformBlock"))
	gl.UniformBlockBinding(r.spriteShader.program, blockIndex, bindingPoint)
	gl.BindBufferBase(gl.UNIFORM_BUFFER, bindingPoint, batchRenderer.fragUbo)
	bindingPoint++

	blockIndex = gl.GetUniformBlockIndex(r.spriteShader.program, glStr("VertexUniformBlock"))
	gl.UniformBlockBinding(r.spriteShader.program, blockIndex, bindingPoint)
	gl.BindBufferBase(gl.UNIFORM_BUFFER, bindingPoint, batchRenderer.vertUbo)
	bindingPoint++

	//blockIndex = gl.GetUniformBlockIndex(r.spriteShader.program, glStr("IndexUniformBlock"))
	//gl.UniformBlockBinding(r.spriteShader.program, blockIndex, bindingPoint)
	//gl.BindBufferBase(gl.UNIFORM_BUFFER, bindingPoint, batchRenderer.indexUbo)
}

func (r *Renderer) InitBatchSpriteShader() {
	directives := []string{"#define MAX_TEXTURE_UNITS " + fmt.Sprintf("%d", batchRenderer.maxTextureUnits-1)}
	r.spriteShader, _ = newShaderProgram(vertShader, fragShader, "", "Main Shader", true, directives)
	r.spriteShader.RegisterAttributes("position", "uv", "index")

	str := []string{"palArray"}
	for i := 0; i < int(batchRenderer.maxTextureUnits)-1; i++ {
		str = append(str, fmt.Sprintf("tex[%d]", i))
	}
	r.spriteShader.RegisterTextures(str...)

}

func RenderSpriteHelper(rp RenderParams) {
	CalculateRenderData(rp)
}

func FillRectHelper(rect [4]int32, color uint32, trans int32) {
	CalculateRectData(rect, color, trans)
}

func PaletteToTextureHelper(pal []uint32) *Texture {
	return PaletteToTextureBatch(pal)
}

func DrawTtfHelper(ttf *TtfFont) {
	BatchParam(&RenderUniformData{isTTF: true, seqNo: batchRenderer.state.curSDRSeqNo, ttf: ttf})
	batchRenderer.state.curSDRSeqNo++
}

func RenderHelper() {
	BatchRender()
}

func RenderCacheResetHelper() {
	batchRenderer.palTexCache = make(map[uint64]*Texture)
	batchRenderer.vertexBufferCache = make(map[uint64]uint32)
	batchRenderer.setInitialUniforms = true
}
