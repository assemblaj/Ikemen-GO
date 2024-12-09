in vec2 position;
in vec2 uv;
in uint index; 
out vec2 texcoord;
flat out uint idx; 

struct VertexUniforms {
	mat4 modelview;
	mat4 projection;
}; 

layout (std140) uniform VertexUniformBlock {
    VertexUniforms vertexUniforms[MAX_UNIFORM_BLOCK_SIZE / 128]; 
};

void unpackIndex(uint packedIndex, out uint vertexUniformIndex, out uint fragUniformIndex, out uint palLayer, out uint texLayer) {
    vertexUniformIndex = (packedIndex >> VERTEX_SHIFT_BATCH) & ((1u << VERTEX_UNIFORM_BITS_BATCH) - 1u);
    fragUniformIndex = (packedIndex >> FRAG_SHIFT_BATCH) & ((1u << FRAG_UNIFORM_BITS_BATCH) - 1u);
    palLayer = (packedIndex >> PAL_SHIFT_BATCH) & ((1u << PAL_LAYER_BITS_BATCH) - 1u);
    texLayer = (packedIndex >> TEX_SHIFT_BATCH) & ((1u << TEX_LAYER_BITS_BATCH) - 1u);
}

void main(void) {
	uint packedIndex = uint(index); 
    
	uint vertexUniformIndex;
    uint fragmentUniformIndex;
    uint palLayer;
    uint texLayer;

    unpackIndex(packedIndex, vertexUniformIndex, fragmentUniformIndex, palLayer, texLayer);

    mat4 modelview = vertexUniforms[vertexUniformIndex].modelview;
    mat4 projection = vertexUniforms[vertexUniformIndex].projection;

	texcoord = uv;
	idx = uint(index);
	gl_Position = projection * (modelview * vec4(position, 0.0, 1.0));
}