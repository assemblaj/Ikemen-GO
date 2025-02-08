//go:build !kinc && gl32

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"runtime"
	"slices"
	"unsafe"

	"github.com/cespare/xxhash"
	mgl "github.com/go-gl/mathgl/mgl32"
)

var batchRenderer BatchRenderer

func init() {
	batchRenderer = NewBatchRenderer()
}

func NewBatchRenderer() BatchRenderer {
	return BatchRenderer{
		vertexBufferCache: make(map[uint64]uint32),
		palTexCache:       make(map[uint64]*Texture),
		state:             NewBatchRenderingState(),
	}
}

func NewBatchRenderingState() BatchRenderingState {
	return BatchRenderingState{
		paramList: make([]RenderUniformData, 0),
	}
}

type BatchRenderer struct {
	spriteVertexBuffer uint32
	// Batch Render
	setInitialUniforms bool
	paletteTex         uint32
	fragUbo            uint32
	vertUbo            uint32
	indexUbo           uint32

	paletteTextureLocation uint32

	maxTextureUnits     int32
	maxUniformBlockSize int32
	vertexUniformMax    int
	fragUniformMax      int
	vertexUniformBits   uint32
	fragUniformBits     uint32
	palLayerBits        uint32
	texLayerBits        uint32
	vertexShift         uint32
	fragShift           uint32
	palShift            uint32
	texShift            uint32

	curVertexBuffer   uint32
	vertexBufferCache map[uint64]uint32
	lastUsedInBatch   RenderUniformData
	palTexCache       map[uint64]*Texture
	curLayer          int32
	curTexLayer       int32
	preFightLayer     int32

	state BatchRenderingState
}
type FragmentUniforms struct {
	x1x2x4x3 [4]float32
	tint     [4]float32
	alpha    float32
	hue      float32
	gray     float32
	add      [4]float32
	mult     [4]float32
	mask     int32
	bitmask  int32
	isFlat   int32
	isRgba   int32
	isTrapez int32
	neg      int32
	padding  [4]byte
}

func (f *FragmentUniforms) Size() int {
	return 96
}

func (f *FragmentUniforms) String() string {
	str := fmt.Sprintf("x1x2x4x3: %v\n", f.x1x2x4x3)
	str = str + fmt.Sprintf("Tint: %v\n", f.tint)
	str = str + fmt.Sprintf("Add: %v\n", f.add)
	str = str + fmt.Sprintf("Alpha: %f\n", f.alpha)
	str = str + fmt.Sprintf("Mult: %v\n", f.mult)
	str = str + fmt.Sprintf("Gray: %f\n", f.gray)
	str = str + fmt.Sprintf("Mask: %d\n", f.mask)
	str = str + fmt.Sprintf("IsFlat: %d\n", f.isFlat)
	str = str + fmt.Sprintf("IsRgba: %d\n", f.isRgba)
	str = str + fmt.Sprintf("IsTropez: %d\n", f.isTrapez)
	str = str + fmt.Sprintf("Neg: %d\n", f.neg)
	str = str + fmt.Sprintf("Hue: %f\n", f.hue)
	return str
}
func (f *FragmentUniforms) createBitMask() uint32 {
	return (uint32(f.isFlat) << 0) |
		(uint32(f.isRgba) << 1) |
		(uint32(f.isTrapez) << 2) |
		(uint32(f.neg) << 3)
}

func (f *FragmentUniforms) ToBytes() []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, f.x1x2x4x3[:])
	err = binary.Write(buf, binary.LittleEndian, f.tint[:])
	err = binary.Write(buf, binary.LittleEndian, f.alpha)
	err = binary.Write(buf, binary.LittleEndian, f.hue)
	err = binary.Write(buf, binary.LittleEndian, f.gray)
	err = binary.Write(buf, binary.LittleEndian, float32(0)) // Padding
	err = binary.Write(buf, binary.LittleEndian, f.add[:])
	err = binary.Write(buf, binary.LittleEndian, f.mult[:])
	err = binary.Write(buf, binary.LittleEndian, f.mask)
	err = binary.Write(buf, binary.LittleEndian, f.createBitMask())
	err = binary.Write(buf, binary.LittleEndian, uint64(0)) // Final padding

	if err != nil {
		log.Fatalf("binary.Write failed: %v", err)
	}
	return buf.Bytes()
}

type VertexUniforms struct {
	Modelview  mgl.Mat4
	Projection mgl.Mat4
}

func (v *VertexUniforms) ToBytes() []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, v.Modelview[:])
	err = binary.Write(buf, binary.LittleEndian, v.Projection[:])

	if err != nil {
		log.Fatalf("binary.Write failed: %v", err)
	}
	return buf.Bytes()
}

// -------- UBO
type IndexUniforms struct {
	FragUniformIndex   int32
	VertexUniformIndex int32
	PalLayer           int32
	TexLayer           int32
}

// packIndex packs an IndexUniforms into a uint32
// func (i *IndexUniforms) packIndex() uint32 {
// 	return uint32((i.VertexUniformIndex & 0x1F) | // 5 bits for VertexUniformIndex
// 		((i.FragUniformIndex & 0x7F) << 5) | // 7 bits for FragUniformIndex
// 		((i.PalLayer & 0x1FF) << 12) | // 9 bits for PalLayer
// 		((i.TexLayer & 0x3F) << 21)) // 6 bits for TexLayer
// }

func (b *BatchRenderer) getIndexConstants(maxUniformBlockSize int, maxTextureImageUnits int) {
	// Calculate dynamic limits and clamp to maximum uint32 range
	vertexUniformMax := MinI(maxUniformBlockSize/128, (1<<32)-1)
	fragUniformMax := MinI(maxUniformBlockSize/96, (1<<32)-1)
	texLayerMax := maxTextureImageUnits // Assume it always fits

	b.vertexUniformMax = vertexUniformMax
	b.fragUniformMax = fragUniformMax

	// Calculate bit requirements
	b.vertexUniformBits = uint32(neededBits(vertexUniformMax))
	b.fragUniformBits = uint32(neededBits(fragUniformMax))
	b.palLayerBits = uint32(9) // Fixed at 512 (2^9)
	b.texLayerBits = uint32(neededBits(texLayerMax))

	// Bit positions (cumulative)
	b.vertexShift = uint32(0)
	b.fragShift = b.vertexUniformBits
	b.palShift = b.fragShift + b.fragUniformBits
	b.texShift = b.palShift + b.palLayerBits
}

func (i *IndexUniforms) packIndex() uint32 {
	return uint32(
		(i.VertexUniformIndex&((1<<batchRenderer.vertexUniformBits)-1))<<batchRenderer.vertexShift |
			(i.FragUniformIndex&((1<<batchRenderer.fragUniformBits)-1))<<batchRenderer.fragShift |
			(i.PalLayer&((1<<batchRenderer.palLayerBits)-1))<<batchRenderer.palShift |
			(i.TexLayer&((1<<batchRenderer.texLayerBits)-1))<<batchRenderer.texShift,
	)
}

// Helper function to calculate the number of bits needed for a value
func neededBits(maxValue int) int {
	bits := 0
	for maxValue > 0 {
		maxValue >>= 1
		bits++
	}
	return bits
}

// Helper function to find the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (i *IndexUniforms) ToBytes() []byte {
	buf := new(bytes.Buffer)

	packedIndex := uint32((i.VertexUniformIndex & 0x1F) | // 5 bits for VertexUniformIndex
		((i.FragUniformIndex & 0x7F) << 5) | // 7 bits for FragUniformIndex
		((i.PalLayer & 0x1FF) << 12) | // 9 bits for PalLayer
		((i.TexLayer & 0x3F) << 21)) // 6 bits for TexL

	err := binary.Write(buf, binary.LittleEndian, packedIndex)

	if err != nil {
		log.Fatalf("binary.Write failed: %v", err)
	}
	return buf.Bytes()
}

type RenderUniformData struct {
	window   [4]int32
	eq       BlendEquation // int
	src, dst BlendFunc     // int
	proj     mgl.Mat4
	tex      uint32
	paltex   uint32
	isRgba   int
	mask     int32
	isTropez int
	isFlat   int

	neg        int
	grayscale  float32
	hue        float32
	padd       [3]float32
	pmul       [3]float32
	tint       [4]float32
	alpha      float32
	modelView  mgl.Mat4
	trans      int32
	invblend   int32
	vertexData []float32
	// Possibly implement later
	//x1x2x4x3        [][]float32
	seqNo           int
	forSprite       bool
	UIMode          bool
	isTTF           bool
	ttf             *TtfFont
	palLayer        int32
	texLayer        int32
	depth           uint8
	vertexDataCache map[uint64]bool
	vertUniforms    VertexUniforms
	fragUniforms    FragmentUniforms
}

type BatchRenderGlobals struct {
	serializeBuffer         bytes.Buffer
	floatConvertBuffer      []byte
	vertexDataBuffer        [][]float32
	vertexDataBufferCounter int
	vertexCacheBuffer       []map[uint64]bool
}

type BatchRenderingState struct {
	paramList    []RenderUniformData
	batchGlobals BatchRenderGlobals
	curSDRSeqNo  int
	maxTextures  int
	uiMode       bool
}

func batchF32Encode(data []float32) []byte {
	// Preallocate the buffer to avoid multiple reallocations.
	requiredCapacity := len(data) * 4 // Each float32 needs 4 bytes.
	if cap(batchRenderer.state.batchGlobals.floatConvertBuffer) < requiredCapacity {
		batchRenderer.state.batchGlobals.floatConvertBuffer = make([]byte, 0, requiredCapacity)
	}
	// Reset the buffer without reallocating memory.
	batchRenderer.state.batchGlobals.floatConvertBuffer = batchRenderer.state.batchGlobals.floatConvertBuffer[:0]

	// Use a single buffer for encoding.
	for _, f := range data {
		u := math.Float32bits(f)
		// Directly append bytes to the buffer.
		batchRenderer.state.batchGlobals.floatConvertBuffer = append(
			batchRenderer.state.batchGlobals.floatConvertBuffer,
			byte(u), byte(u>>8), byte(u>>16), byte(u>>24),
		)
	}
	return batchRenderer.state.batchGlobals.floatConvertBuffer
}

func BatchRender() {
	uniqueFrags := make([]FragmentUniforms, 0)
	uniqueVertexData := make([]VertexUniforms, 0)

	var currentBatch []RenderUniformData
	var lastHash uint64 = 0

	vertices := make([]float32, 0, 1024)

	// Aggregate all vertex data.
	for _, entry := range batchRenderer.state.paramList {
		vertices = append(vertices, entry.vertexData...)
	}

	if len(vertices) == 0 {
		return
	}

	// 'first' tracks the starting vertex index of the current batch.
	var first int32 = 0
	// 'count' will now be calculated per batch within the loop.
	var count int32 = 0
	totalVertices := len(vertices) / 4
	indexUniforms := make([]IndexUniforms, 0, totalVertices)
	packedIndexUniforms := make([]uint32, 0, totalVertices)

	for i, entry := range batchRenderer.state.paramList {
		if i == 0 {
			uniqueFrags = append(uniqueFrags, entry.fragUniforms)
			uniqueVertexData = append(uniqueVertexData, entry.vertUniforms)
		} else {
			if !slices.Contains(uniqueFrags, entry.fragUniforms) {
				uniqueFrags = append(uniqueFrags, entry.fragUniforms)
			}
			if !slices.Contains(uniqueVertexData, entry.vertUniforms) {
				uniqueVertexData = append(uniqueVertexData, entry.vertUniforms)
			}
		}
	}
	batches := make([][]RenderUniformData, 0)

	for i, entry := range batchRenderer.state.paramList {
		data, _ := entry.Serialize()
		currentHash := xxhash.Sum64(data)

		// Start a new batch if the hash has changed and currentBatch is not empty
		if i == 0 || currentHash != lastHash {
			if len(currentBatch) > 0 {
				batches = append(batches, currentBatch)
				currentBatch = []RenderUniformData{}
			}
			lastHash = currentHash
		}
		currentBatch = append(currentBatch, entry)

		// Append the final batch after the last entry
		if i == len(batchRenderer.state.paramList)-1 && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
		}
	}
	batches = processBatches(batches)

	for _, batch := range batches {
		textures := getUniqueTextures(batch)
		for _, entry := range batch {
			numVertices := int32(len(entry.vertexData) / 4)
			var fragIndex int
			for j := 0; j < len(uniqueFrags); j++ {
				if uniqueFrags[j] == entry.fragUniforms {
					fragIndex = j
				}
			}
			var vertexIndex int
			for j := 0; j < len(uniqueVertexData); j++ {
				if uniqueVertexData[j] == entry.vertUniforms {
					vertexIndex = j
				}
			}
			var texIndex int
			for j := 0; j < len(textures); j++ {
				if textures[j] == entry.tex {
					texIndex = j
				}
			}

			indexUniform := IndexUniforms{}
			indexUniform.PalLayer = entry.palLayer
			indexUniform.TexLayer = int32(texIndex)
			indexUniform.FragUniformIndex = int32(fragIndex)
			indexUniform.VertexUniformIndex = int32(vertexIndex)
			for i := 0; i < int(numVertices); i++ {
				indexUniforms = append(indexUniforms, indexUniform)
				packedIndexUniforms = append(packedIndexUniforms, indexUniform.packIndex())
			}
		}
	}
	// maxVertices := 10000
	// batchCycles := (len(vertices) / 4) / maxVertices

	// vertexBytes := batchF32Encode(vertices)
	// result := appendUint32ToByteFloats(vertexBytes, packedIndexUniforms)

	// gfx.SetVertexBytes(result)

	// gfx.UploadFragmentUBO(uniqueFrags)
	// gfx.UploadVertexUBO(uniqueVertexData)
	// gfx.BindUBOs()
	// first = 0
	// count = 0

	// for _, batch := range batches {
	// 	count = int32(getNumVertices2(batch))
	// 	if count > 0 || batch[0].isTTF {
	// 		processBatchOptimized(batch, first, count)
	// 	}
	// 	first += count
	// }
	maxVertices := 50000

	// Break vertices into manageable chunks based on maxVertices
	chunkSize := maxVertices * 4 // Assuming 4 floats per vertex
	numChunks := (len(vertices) + chunkSize - 1) / chunkSize

	first = 0
	for chunk := 0; chunk < numChunks; chunk++ {
		// Calculate the range for this chunk
		start := chunk * chunkSize
		end := start + chunkSize
		if end > len(vertices) {
			end = len(vertices)
		}

		// Prepare the chunk of vertices
		vertexChunk := vertices[start:end]
		vertexBytes := batchF32Encode(vertexChunk)
		result := appendUint32ToByteFloats(vertexBytes, packedIndexUniforms[start/4:end/4])

		// Upload the chunk to the GPU
		gfx.SetVertexBytes(result)

		// Upload UBOs and bind them for this batch
		gfx.UploadFragmentUBO(uniqueFrags)
		gfx.UploadVertexUBO(uniqueVertexData)
		gfx.BindUBOs()

		count = int32(0)

		// Process each batch within the current chunk
		for _, batch := range batches {
			batchStart := first
			count = int32(getNumVertices2(batch))

			if count > 0 || batch[0].isTTF {
				processBatchOptimized(batch, batchStart, count)
			}

			first += count

			// Stop processing batches if we've reached the end of this chunk
			if first >= int32(end) {
				break
			}
		}
	}

	// Reset state after processing.
	for i := 0; i < len(batchRenderer.state.batchGlobals.vertexDataBuffer); i++ {
		batchRenderer.state.batchGlobals.vertexDataBuffer[i] = batchRenderer.state.batchGlobals.vertexDataBuffer[i][:0]
		for key := range batchRenderer.state.batchGlobals.vertexCacheBuffer[i] {
			delete(batchRenderer.state.batchGlobals.vertexCacheBuffer[i], key)
		}
	}
	batchRenderer.state.paramList = batchRenderer.state.paramList[:0]
	batchRenderer.state.batchGlobals.vertexDataBufferCounter = 0
	batchRenderer.state.curSDRSeqNo = 0
}

func appendUint32ToByteFloats(floatBytes []byte, uintData []uint32) []byte {
	// Ensure the floatBytes length is a multiple of 16 (4 floats)
	if len(floatBytes)%16 != 0 {
		panic("floatBytes length must be a multiple of 16 (4 floats per uint32)")
	}

	// Ensure there are enough uint32 values
	if len(uintData)*16 != len(floatBytes) {
		panic("uintData must have enough entries to match every 4 floats in floatBytes")
	}

	// Create a buffer to hold the resulting byte data
	var result bytes.Buffer

	floatIndex := 0
	uintIndex := 0
	for floatIndex < len(floatBytes) {
		// Write 16 bytes (4 floats)
		result.Write(floatBytes[floatIndex : floatIndex+16])
		floatIndex += 16

		// Write 4 bytes (1 uint32)
		binary.Write(&result, binary.LittleEndian, uintData[uintIndex])
		uintIndex++
	}

	return result.Bytes()
}

func processBatches(batches [][]RenderUniformData) [][]RenderUniformData {
	var finalBatches [][]RenderUniformData
	for _, batch := range batches {
		var subBatch []RenderUniformData
		uniqueTextures := make(map[uint32]bool)
		for _, entry := range batch {
			if _, exists := uniqueTextures[entry.tex]; !exists {
				if len(uniqueTextures) == int(batchRenderer.maxTextureUnits-1) {
					finalBatches = append(finalBatches, subBatch)
					subBatch = []RenderUniformData{}
					uniqueTextures = make(map[uint32]bool)
				}
				uniqueTextures[entry.tex] = true
			}
			subBatch = append(subBatch, entry)
		}
		if len(subBatch) > 0 {
			finalBatches = append(finalBatches, subBatch)
		}
	}
	return finalBatches
}

func getNumVertices(batch []RenderUniformData) int {
	totalVertices := 0
	for _, entry := range batch {
		totalVertices += len(entry.vertexData)
	}
	return totalVertices / 4
}

func getNumVertices2(batch []RenderUniformData) int {
	totalVertices := 0
	for _, entry := range batch {
		vertexCount := len(entry.vertexData)
		totalVertices += vertexCount

		// Add 1 to totalVertices after every 4 vertices
		totalVertices += vertexCount / 4
	}

	// Divide totalVertices by 5
	return totalVertices / 5
}

func getUniqueTextures(batch []RenderUniformData) []uint32 {
	uniqueTextures := make([]uint32, 0, 15)
	for i := 0; i < len(batch); i++ {
		if !slices.Contains(uniqueTextures, batch[i].tex) {
			uniqueTextures = append(uniqueTextures, batch[i].tex)
		}
	}
	return uniqueTextures
}

func processBatchOptimized(batch []RenderUniformData, start int32, total int32) {
	if len(batch) == 0 {
		return
	}
	uniqueTextures := make([]uint32, 0, 15)
	for i := 0; i < len(batch); i++ {
		if !slices.Contains(uniqueTextures, batch[i].tex) {
			uniqueTextures = append(uniqueTextures, batch[i].tex)
		}
	}

	srd := batch[0]
	if srd.isTTF {
		(*srd.ttf).PrintBatch()
		return
	}

	//UIMode = srd.UIMode
	if runtime.GOOS == "darwin" || srd.forSprite {
		gfx.Scissor(srd.window[0], srd.window[1], srd.window[2], srd.window[3])
	}
	gfx.SetPipeline(srd.eq, srd.src, srd.dst)

	var names []string
	if srd.forSprite {
		gfx.SetTextureArrayWithHandle("palArray", batchRenderer.paletteTex)
		names = make([]string, len(uniqueTextures))
		for i, h := range uniqueTextures {
			str := fmt.Sprintf("tex[%d]", i)
			names[i] = str
			gfx.SetTextureWithHandle(str, h)
		}
	}

	gfx.RenderQuadBatchAtIndex(start, total)

	if srd.forSprite && len(uniqueTextures) > 0 {
		gfx.UnbindTextures(names)
	}
	gfx.ReleasePipeline()

	if srd.forSprite {
		gfx.DisableScissor()
	}
}

func (r *RenderUniformData) Serialize() ([]byte, error) {
	buf := &batchRenderer.state.batchGlobals.serializeBuffer
	buf.Reset()

	if err := binary.Write(buf, binary.LittleEndian, r.window[:]); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, int32(r.eq)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, int32(r.src)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, int32(r.dst)); err != nil {
		return nil, err
	}
	// if err := binary.Write(buf, binary.LittleEndian, int32(r.isRgba)); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.mask); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, int32(r.isTropez)); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, int32(r.isFlat)); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, int32(r.neg)); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.grayscale); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.hue); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.padd[:]); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.pmul[:]); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.tint[:]); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.alpha); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.modelView[:]); err != nil {
	// 	return nil, err
	// }
	// if err := binary.Write(buf, binary.LittleEndian, r.trans); err != nil {
	// 	return nil, err
	// }
	// // if err := binary.Write(buf, binary.LittleEndian, r.invblend); err != nil {
	// // 	return nil, err
	// // }
	if err := binary.Write(buf, binary.LittleEndian, r.palLayer); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, r.isTTF); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewRenderUniformData() RenderUniformData {
	rud := RenderUniformData{}
	if len(batchRenderer.state.batchGlobals.vertexDataBuffer) == 0 {
		batchRenderer.state.batchGlobals.vertexDataBuffer = make([][]float32, 256)

		for i := 0; i < 256; i++ {
			batchRenderer.state.batchGlobals.vertexDataBuffer[i] = make([]float32, 0, 24)
		}
	}
	if len(batchRenderer.state.batchGlobals.vertexCacheBuffer) == 0 {
		batchRenderer.state.batchGlobals.vertexCacheBuffer = make([]map[uint64]bool, 256)

		for i := 0; i < 256; i++ {
			batchRenderer.state.batchGlobals.vertexCacheBuffer[i] = make(map[uint64]bool)
		}
	}

	if len(batchRenderer.state.batchGlobals.vertexDataBuffer) > batchRenderer.state.batchGlobals.vertexDataBufferCounter {
		rud.vertexData = batchRenderer.state.batchGlobals.vertexDataBuffer[batchRenderer.state.batchGlobals.vertexDataBufferCounter]
		rud.vertexDataCache = batchRenderer.state.batchGlobals.vertexCacheBuffer[batchRenderer.state.batchGlobals.vertexDataBufferCounter]
		batchRenderer.state.batchGlobals.vertexDataBufferCounter++
	} else {
		batchRenderer.state.batchGlobals.vertexDataBuffer = append(batchRenderer.state.batchGlobals.vertexDataBuffer, make([]float32, 0, 24))
		rud.vertexData = batchRenderer.state.batchGlobals.vertexDataBuffer[batchRenderer.state.batchGlobals.vertexDataBufferCounter]
		batchRenderer.state.batchGlobals.vertexCacheBuffer = append(batchRenderer.state.batchGlobals.vertexCacheBuffer, make(map[uint64]bool))
		rud.vertexDataCache = batchRenderer.state.batchGlobals.vertexCacheBuffer[batchRenderer.state.batchGlobals.vertexDataBufferCounter]
		batchRenderer.state.batchGlobals.vertexDataBufferCounter++
	}
	return rud
}

func CalculateRenderData(rp RenderParams) {
	if !rp.IsValid() {
		return
	}

	rmInitSub(&rp)

	rd := NewRenderUniformData()
	rd.forSprite = true
	rd.UIMode = batchRenderer.state.uiMode

	neg, grayscale, padd, pmul, invblend, hue := false, float32(0), [3]float32{0, 0, 0}, [3]float32{1, 1, 1}, int32(0), float32(0)
	tint := [4]float32{float32(rp.tint&0xff) / 255, float32(rp.tint>>8&0xff) / 255,
		float32(rp.tint>>16&0xff) / 255, float32(rp.tint>>24&0xff) / 255}

	if rp.pfx != nil {
		blending := rp.trans
		//if rp.trans == -2 || rp.trans == -1 || (rp.trans&0xff > 0 && rp.trans>>10&0xff >= 255) {
		//	blending = true
		//}
		neg, grayscale, padd, pmul, invblend, hue = rp.pfx.getFcPalFx(false, int(blending))
		//if rp.trans == -2 && invblend < 1 {
		//padd[0], padd[1], padd[2] = -padd[0], -padd[1], -padd[2]
		//}
	}

	proj := mgl.Ortho(0, float32(sys.scrrect[2]), 0, float32(sys.scrrect[3]), -65535, 65535)
	modelview := mgl.Translate3D(0, float32(sys.scrrect[3]), 0)
	rd.window = *rp.window

	// gfx.Scissor(rp.window[0], rp.window[1], rp.window[2], rp.window[3])
	renderWithBlending(func(eq BlendEquation, src, dst BlendFunc, a float32) {
		rd.tex = rp.tex.handle
		rd.texLayer = rp.tex.layer
		rd.eq = eq
		rd.src = src
		rd.dst = dst
		rd.proj = proj
		rd.tex = rp.tex.handle
		if rp.paltex == nil {
			rd.isRgba = 1
		} else {
			//rd.paltex = rp.paltex.handle
			rd.palLayer = rp.paltex.layer
			rd.isRgba = 0
		}
		rd.mask = rp.mask
		rd.isTropez = int(Btoi(AbsF(AbsF(rp.xts)-AbsF(rp.xbs)) > 0.001))
		rd.isFlat = 0
		rd.neg = int(Btoi(neg))
		rd.grayscale = grayscale
		rd.hue = hue
		rd.padd = padd
		rd.pmul = pmul
		rd.tint = tint
		rd.alpha = a
		//rd.modelView = modelview
		//rd.trans = rp.trans
		//rd.invblend = invblend
		if rp.paltex == nil {
			rd.fragUniforms.isRgba = 1
		} else {
			//rd.paltex = rp.paltex.handle
			rd.palLayer = rp.paltex.layer
			rd.fragUniforms.isRgba = 0
		}
		rd.fragUniforms.mask = rp.mask
		rd.fragUniforms.isTrapez = int32(Btoi(AbsF(AbsF(rp.xts)-AbsF(rp.xbs)) > 0.001))
		rd.fragUniforms.isFlat = 0
		rd.fragUniforms.neg = int32(Btoi(neg))
		rd.fragUniforms.gray = grayscale
		rd.fragUniforms.hue = hue
		rd.fragUniforms.add = [4]float32{padd[0], padd[1], padd[2], 0}
		rd.fragUniforms.mult = [4]float32{pmul[0], pmul[1], pmul[2], 0}
		rd.fragUniforms.tint = tint
		rd.fragUniforms.alpha = a
		rd.trans = rp.trans
		rd.vertUniforms.Projection = proj
		rmTileSubBatch(modelview, rp, &rd)
		rd.seqNo = batchRenderer.state.curSDRSeqNo
		batchRenderer.state.curSDRSeqNo++
		BatchParam(&rd)
	}, rp.trans, rp.paltex != nil, invblend, &neg, &padd, &pmul, rp.paltex == nil)
}

func BatchParam(rp *RenderUniformData) {
	if rp != nil {
		batchRenderer.state.paramList = append(batchRenderer.state.paramList, *rp)
	}
}

func (r *RenderUniformData) AppendVertexData(vertices []float32) {
	data := batchF32Encode(vertices)

	hash := xxhash.Sum64(data)
	if _, ok := r.vertexDataCache[hash]; !ok {
		r.vertexData = append(r.vertexData, vertices...)
		r.vertexDataCache[hash] = true
	} else {
		// fmt.Println("I'm actually using the cache.")
	}
}

// Render a quad with optional horizontal tiling
func rmTileHSubBatch2(modelview mgl.Mat4, x1, y1, x2, y2, x3, y3, x4, y4, dy, width float32, rp RenderParams, rd *RenderUniformData) {
	//            p3
	//    p4 o-----o-----o- - -o
	//      /      |      \     ` .
	//     /       |       \       `.
	//    o--------o--------o- - - - o
	//   p1         p2
	topdist := (x3 - x4) * (((float32(rp.tile.xspacing) + width) / rp.xas) / width)
	botdist := (x2 - x1) * (((float32(rp.tile.xspacing) + width) / rp.xas) / width)
	if AbsF(topdist) >= 0.01 {
		db := (x4 - rp.rcx) * (botdist - topdist) / AbsF(topdist)
		x1 += db
		x2 += db
	}

	// Compute left/right tiling bounds (or right/left when topdist < 0)
	xmax := float32(sys.scrrect[2])
	left, right := int32(0), int32(1)
	if rp.tile.xflag != 0 {
		if topdist >= 0.01 {
			left = 1 - int32(math.Ceil(float64(MaxF(x3/topdist, x2/botdist))))
			right = int32(math.Ceil(float64(MaxF((xmax-x4)/topdist, (xmax-x1)/botdist))))
		} else if topdist <= -0.01 {
			left = 1 - int32(math.Ceil(float64(MaxF((xmax-x3)/-topdist, (xmax-x2)/-botdist))))
			right = int32(math.Ceil(float64(MaxF(x4/-topdist, x1/-botdist))))
		}
		if rp.tile.xflag != 1 {
			left = 0
			right = Min(right, Max(rp.tile.xflag, 1))
		}
	}

	// Draw all quads in one loop
	for n := left; n < right; n++ {
		x1d, x2d := x1+float32(n)*botdist, x2+float32(n)*botdist
		x3d, x4d := x3+float32(n)*topdist, x4+float32(n)*topdist
		mat := modelview
		if !rp.rot.IsZero() {
			mat = mat.Mul4(mgl.Translate3D(rp.rcx+float32(n)*botdist, rp.rcy+dy, 0))
			//modelview = modelview.Mul4(mgl.Scale3D(1, rp.vs, 1))
			mat = mat.Mul4(mgl.Rotate3DZ(rp.rot.angle * math.Pi / 180.0).Mat4())
			mat = mat.Mul4(mgl.Translate3D(-(rp.rcx + float32(n)*botdist), -(rp.rcy + dy), 0))
		}

		drawQuadsBatch(rd, mat, x1d, y1, x2d, y2, x3d, y3, x4d, y4)
	}
}

func rmTileSubBatch(modelview mgl.Mat4, rp RenderParams, rd *RenderUniformData) {
	x1, y1 := rp.x, rp.rcy+((rp.y-rp.ys*float32(rp.size[1]))-rp.rcy)*rp.vs
	x2, y2 := x1+rp.xbs*float32(rp.size[0]), y1
	x3, y3 := rp.x+rp.xts*float32(rp.size[0]), rp.rcy+(rp.y-rp.rcy)*rp.vs
	x4, y4 := rp.x, y3
	//var pers float32
	//if AbsF(rp.xts) < AbsF(rp.xbs) {
	//	pers = AbsF(rp.xts) / AbsF(rp.xbs)
	//} else {
	//	pers = AbsF(rp.xbs) / AbsF(rp.xts)
	//}
	if !rp.rot.IsZero() && rp.tile.xflag == 0 && rp.tile.yflag == 0 {

		if rp.vs != 1 {
			y1 = rp.rcy + ((rp.y - rp.ys*float32(rp.size[1])) - rp.rcy)
			y2 = y1
			y3 = rp.y
			y4 = y3
		}
		if rp.projectionMode == 0 {
			modelview = modelview.Mul4(mgl.Translate3D(rp.rcx, rp.rcy, 0))
		} else if rp.projectionMode == 1 {
			// This is the inverse of the orthographic projection matrix
			matrix := mgl.Mat4{float32(sys.scrrect[2] / 2.0), 0, 0, 0, 0, float32(sys.scrrect[3] / 2), 0, 0, 0, 0, -65535, 0, float32(sys.scrrect[2] / 2), float32(sys.scrrect[3] / 2), 0, 1}
			modelview = modelview.Mul4(mgl.Translate3D(0, -float32(sys.scrrect[3]), rp.fLength))
			modelview = modelview.Mul4(matrix)
			modelview = modelview.Mul4(mgl.Frustum(-float32(sys.scrrect[2])/2/rp.fLength, float32(sys.scrrect[2])/2/rp.fLength, -float32(sys.scrrect[3])/2/rp.fLength, float32(sys.scrrect[3])/2/rp.fLength, 1.0, 65535))
			modelview = modelview.Mul4(mgl.Translate3D(-float32(sys.scrrect[2])/2.0, float32(sys.scrrect[3])/2.0, -rp.fLength))
			modelview = modelview.Mul4(mgl.Translate3D(rp.rcx, rp.rcy, 0))
		} else if rp.projectionMode == 2 {
			matrix := mgl.Mat4{float32(sys.scrrect[2] / 2.0), 0, 0, 0, 0, float32(sys.scrrect[3] / 2), 0, 0, 0, 0, -65535, 0, float32(sys.scrrect[2] / 2), float32(sys.scrrect[3] / 2), 0, 1}
			//modelview = modelview.Mul4(mgl.Translate3D(0, -float32(sys.scrrect[3]), 2048))
			modelview = modelview.Mul4(mgl.Translate3D(rp.rcx-float32(sys.scrrect[2])/2.0-rp.xOffset, rp.rcy-float32(sys.scrrect[3])/2.0+rp.yOffset, rp.fLength))
			modelview = modelview.Mul4(matrix)
			modelview = modelview.Mul4(mgl.Frustum(-float32(sys.scrrect[2])/2/rp.fLength, float32(sys.scrrect[2])/2/rp.fLength, -float32(sys.scrrect[3])/2/rp.fLength, float32(sys.scrrect[3])/2/rp.fLength, 1.0, 65535))
			modelview = modelview.Mul4(mgl.Translate3D(rp.xOffset, -rp.yOffset, -rp.fLength))
		}

		// Apply shear matrix before rotation
		shearMatrix := mgl.Mat4{
			1, 0, 0, 0,
			rp.rxadd, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, 1}
		modelview = modelview.Mul4(shearMatrix)
		modelview = modelview.Mul4(mgl.Translate3D(rp.rxadd*rp.ys*float32(rp.size[1]), 0, 0))

		modelview = modelview.Mul4(mgl.Scale3D(1, rp.vs, 1))
		modelview = modelview.Mul4(
			mgl.Rotate3DX(-rp.rot.xangle * math.Pi / 180.0).Mul3(
				mgl.Rotate3DY(rp.rot.yangle * math.Pi / 180.0)).Mul3(
				mgl.Rotate3DZ(rp.rot.angle * math.Pi / 180.0)).Mat4())
		modelview = modelview.Mul4(mgl.Translate3D(-rp.rcx, -rp.rcy, 0))

		drawQuadsBatch(rd, modelview, x1, y1, x2, y2, x3, y3, x4, y4)
		return
	}
	if rp.tile.yflag == 1 && rp.xbs != 0 {
		x1 += rp.rxadd * rp.ys * float32(rp.size[1])
		x2 = x1 + rp.xbs*float32(rp.size[0])
		x1d, y1d, x2d, y2d, x3d, y3d, x4d, y4d := x1, y1, x2, y2, x3, y3, x4, y4
		n := 0
		var xy []float32
		for {
			x1d, y1d = x4d, y4d+rp.ys*rp.vs*((float32(rp.tile.yspacing)+float32(rp.size[1]))/rp.yas-float32(rp.size[1]))
			x2d, y2d = x3d, y1d
			x3d = x4d - rp.rxadd*rp.ys*float32(rp.size[1]) + (rp.xts/rp.xbs)*(x3d-x4d)
			y3d = y2d + rp.ys*rp.vs*float32(rp.size[1])
			x4d = x4d - rp.rxadd*rp.ys*float32(rp.size[1])
			if AbsF(y3d-y4d) < 0.01 {
				break
			}
			y4d = y3d
			if rp.ys*((float32(rp.tile.yspacing)+float32(rp.size[1]))/rp.yas) < 0 {
				if y1d <= float32(-sys.scrrect[3]) && y4d <= float32(-sys.scrrect[3]) {
					break
				}
			} else if y1d >= 0 && y4d >= 0 {
				break
			}
			n += 1
			xy = append(xy, x1d, x2d, x3d, x4d, y1d, y2d, y3d, y4d)
		}
		for {
			if len(xy) == 0 {
				break
			}
			x1d, x2d, x3d, x4d, y1d, y2d, y3d, y4d, xy = xy[len(xy)-8], xy[len(xy)-7], xy[len(xy)-6], xy[len(xy)-5], xy[len(xy)-4], xy[len(xy)-3], xy[len(xy)-2], xy[len(xy)-1], xy[:len(xy)-8]
			if (0 > y1d || 0 > y4d) &&
				(y1d > float32(-sys.scrrect[3]) || y4d > float32(-sys.scrrect[3])) {
				rmTileHSubBatch2(modelview, x1d, y1d, x2d, y2d, x3d, y3d, x4d, y4d, y1d-y1, float32(rp.size[0]), rp, rd)
			}
		}
	}
	if rp.tile.yflag == 0 || rp.xts != 0 {
		x1 += rp.rxadd * rp.ys * float32(rp.size[1])
		x2 = x1 + rp.xbs*float32(rp.size[0])
		n := rp.tile.yflag
		oy := y1
		for {
			if rp.ys*((float32(rp.tile.yspacing)+float32(rp.size[1]))/rp.yas) > 0 {
				if y1 <= float32(-sys.scrrect[3]) && y4 <= float32(-sys.scrrect[3]) {
					break
				}
			} else if y1 >= 0 && y4 >= 0 {
				break
			}
			if (0 > y1 || 0 > y4) &&
				(y1 > float32(-sys.scrrect[3]) || y4 > float32(-sys.scrrect[3])) {
				rmTileHSubBatch2(modelview, x1, y1, x2, y2, x3, y3, x4, y4, y1-oy,
					float32(rp.size[0]), rp, rd)
			}
			if rp.tile.yflag != 1 && n != 0 {
				n--
			}
			if n == 0 {
				break
			}
			x4, y4 = x1, y1-rp.ys*rp.vs*((float32(rp.tile.yspacing)+float32(rp.size[1]))/rp.yas-float32(rp.size[1]))
			x3, y3 = x2, y4
			x2 = x1 + rp.rxadd*rp.ys*float32(rp.size[1]) + (rp.xbs/rp.xts)*(x2-x1)
			y2 = y3 - rp.ys*rp.vs*float32(rp.size[1])
			x1 = x1 + rp.rxadd*rp.ys*float32(rp.size[1])
			if AbsF(y1-y2) < 0.01 {
				break
			}
			y1 = y2
		}
	}
}

func drawQuadsBatch(rd *RenderUniformData, modelview mgl.Mat4, x1, y1, x2, y2, x3, y3, x4, y4 float32) {
	if rd == nil {
		gfx.SetUniformMatrix("modelview", modelview[:])
		gfx.SetUniformF("x1x2x4x3", x1, x2, x4, x3) // this uniform is optional
		gfx.SetVertexData(
			x2, y2, 1, 1,
			x3, y3, 1, 0,
			x1, y1, 0, 1,
			x4, y4, 0, 0)

		gfx.RenderQuad()
	} else {
		rd.AppendVertexData([]float32{
			x2, y2, 1, 1,
			x3, y3, 1, 0,
			x1, y1, 0, 1,

			x1, y1, 0, 1,
			x3, y3, 1, 0,
			x4, y4, 0, 0,
		})
		rd.fragUniforms.x1x2x4x3 = [4]float32{x1, x2, x4, x3}
		rd.modelView = modelview
		rd.vertUniforms.Modelview = modelview
	}
}

func CalculateRectData(rect [4]int32, color uint32, trans int32) {
	rd := NewRenderUniformData()
	rd.UIMode = batchRenderer.state.uiMode

	r := float32(color>>16&0xff) / 255
	g := float32(color>>8&0xff) / 255
	b := float32(color&0xff) / 255

	modelview := mgl.Translate3D(0, float32(sys.scrrect[3]), 0)
	proj := mgl.Ortho(0, float32(sys.scrrect[2]), 0, float32(sys.scrrect[3]), -65535, 65535)

	x1, y1 := float32(rect[0]), -float32(rect[1])
	x2, y2 := float32(rect[0]+rect[2]), -float32(rect[1]+rect[3])

	renderWithBlending(func(eq BlendEquation, src, dst BlendFunc, a float32) {

		rd.eq = eq
		rd.src = src
		rd.dst = dst
		// rd.vertexData = append(rd.vertexData, []float32{
		// 	x1, y2, 0, 1,
		// 	x1, y1, 0, 0,
		// 	x2, y1, 1, 0,

		// 	x1, y2, 0, 1,
		// 	x2, y1, 1, 0,
		// 	x2, y2, 1, 1,
		// }...)
		rd.AppendVertexData([]float32{
			x1, y2, 0, 1,
			x1, y1, 0, 0,
			x2, y1, 1, 0,

			x1, y2, 0, 1,
			x2, y1, 1, 0,
			x2, y2, 1, 1,
		})
		rd.modelView = modelview
		rd.proj = proj
		rd.isFlat = 1
		rd.tint = [4]float32{r, g, b, a}
		rd.trans = trans
		rd.invblend = 0

		rd.fragUniforms.isFlat = 1
		rd.fragUniforms.tint = [4]float32{r, g, b, a}
		rd.vertUniforms.Projection = proj
		rd.vertUniforms.Modelview = modelview
		rd.seqNo = batchRenderer.state.curSDRSeqNo
		batchRenderer.state.curSDRSeqNo++
		BatchParam(&rd)
	}, trans, true, 0, nil, nil, nil, false)
}

func PaletteToTextureSub(pal []uint32) *Texture {
	//tx := newTexture(256, 1, 32, false)
	//tx.SetData(unsafe.Slice((*byte)(unsafe.Pointer(&pal[0])), len(pal)*4))
	tx := newTextureLayer(256, 1, 32, batchRenderer.curLayer, false)
	tx.SetDataArray(batchRenderer.curLayer, batchRenderer.paletteTex, unsafe.Slice((*byte)(unsafe.Pointer(&pal[0])), len(pal)*4))
	batchRenderer.curLayer++
	return tx
}

func PaletteToTextureBatch(pal []uint32) *Texture {
	return GenerateTextureFromPalette(pal)
}

func GenerateTextureFromPalette(pal []uint32) *Texture {
	key := HashPalette(pal)

	if texture, exists := batchRenderer.palTexCache[key]; exists {
		return texture
	}

	newTexture := PaletteToTextureSub(pal)
	batchRenderer.palTexCache[key] = newTexture
	return newTexture
}

func uint32SliceToBytes(slice []uint32) []byte {
	var buf bytes.Buffer
	for _, val := range slice {
		if err := binary.Write(&buf, binary.LittleEndian, val); err != nil {
			return nil
		}
	}
	return buf.Bytes()
}

func HashPalette(pal []uint32) uint64 {
	return xxhash.Sum64(uint32SliceToBytes(pal))
}
