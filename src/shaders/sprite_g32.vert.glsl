in vec2 position;
in vec2 uv;
in float index; 
out vec2 texcoord;
flat out uint idx; 

struct VertexUniforms {
	mat4 modelview;
	mat4 projection;
}; 

layout (std140) uniform VertexUniformBlock {
    VertexUniforms vertexUniforms[32]; 
};

void main(void) {
	uint packedIndex = uint(index); 
	uint vertexUniformIndex = packedIndex & uint(0x1F);                 

    mat4 modelview = vertexUniforms[vertexUniformIndex].modelview;
    mat4 projection = vertexUniforms[vertexUniformIndex].projection;

	texcoord = uv;
	idx = uint(index);
	gl_Position = projection * (modelview * vec4(position, 0.0, 1.0));
}