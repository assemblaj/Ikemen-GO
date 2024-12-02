const uint FLAT_BIT = uint(0);     // isFlat
const uint RGBA_BIT = uint(1);     // isRgba
const uint TRAPEZ_BIT = uint(2);   // isTrapez
const uint NEG_BIT = uint(3);      // neg

// The maximum texture units of the system - 1 for the palette texture array 
#ifdef MAX_TEXTURE_UNITS
	uniform sampler2D tex[MAX_TEXTURE_UNITS];  
#else 
	uniform sampler2D tex[15];
#endif 
uniform sampler2DArray palArray;

// Structured in this weird way to avoid padding 
struct FragmentUniforms {
    vec4 x1x2x4x3;
    vec4 tint;
    float alpha;
    float hue;
    float gray;
    vec4 add;
    vec4 mult;
    int mask;
	uint bitmask; 
};

layout (std140) uniform FragmentUniformBlock {
    FragmentUniforms fragmentUniforms[128]; 
};

in vec2 texcoord;
flat in uint idx;
out vec4 FragColor;

vec3 hue_shift(vec3 color, float dhue) {
	float s = sin(dhue);
	float c = cos(dhue);
	return (color * c) + (color * s) * mat3(
		vec3(0.167444, 0.329213, -0.496657),
		vec3(-0.327948, 0.035669, 0.292279),
		vec3(1.250268, -1.047561, -0.202707)
	) + dot(vec3(0.299, 0.587, 0.114), color) * (1.0 - c);
}

void main(void) {

	uint packedIndex = idx; 

    uint vertexUniformIndex = packedIndex & uint(0x1F);              // 5 bits for VertexUniformIndex
    uint fragmentUniformIndex = (packedIndex >> 5) & uint(0x7F);     // 7 bits for FragUniformIndex
    uint palLayer = (packedIndex >> 12) & uint(0x1FF);               // 9 bits for PalLayer
    uint texLayer = (packedIndex >> 21) & uint(0x3F);                // 6 bits for TexLayer

    vec4 x1x2x4x3 = fragmentUniforms[fragmentUniformIndex].x1x2x4x3;
	vec4 tint = fragmentUniforms[fragmentUniformIndex].tint;
    vec3 add = fragmentUniforms[fragmentUniformIndex].add.xyz;
    float alpha =   fragmentUniforms[fragmentUniformIndex].alpha;
    vec3 mult = fragmentUniforms[fragmentUniformIndex].mult.xyz;
    float gray =   fragmentUniforms[fragmentUniformIndex].gray;
    int mask = fragmentUniforms[fragmentUniformIndex].mask;
	uint bitmask = fragmentUniforms[fragmentUniformIndex].bitmask;
	bool isFlat = (bitmask & (uint(1) << FLAT_BIT)) != uint(0);
	bool isRgba = (bitmask & (uint(1) << RGBA_BIT)) != uint(0);
	bool isTrapez = (bitmask & (uint(1) << TRAPEZ_BIT)) != uint(0);
	bool neg = (bitmask & (uint(1) << NEG_BIT)) != uint(0);
    float hue =   fragmentUniforms[fragmentUniformIndex].hue;

	if (isFlat) {
		FragColor = tint;
	} else {
		vec2 uv = texcoord;
		if (isTrapez) {
			// Compute left/right trapezoid bounds at height uv.y
			vec2 bounds = mix(x1x2x4x3.zw, x1x2x4x3.xy, uv.y);
			// Correct uv.x from the fragment position on that segment
			uv.x = (gl_FragCoord.x - bounds[0]) / (bounds[1] - bounds[0]);
		}

		//vec4 c = texture2D(tex[int(texLayer)], uv);
		vec4 c = get_tex(int(texLayer), tex, uv);
		
		vec3 neg_base = vec3(1.0);
		vec3 final_add = add;
		vec4 final_mul = vec4(mult, alpha);
		if (isRgba) {
			if (mask == -1) {
				c.a = 1.0;
			}			
			// RGBA sprites use premultiplied alpha for transparency	
			neg_base *= c.a;
			final_add *= c.a;
			final_mul.rgb *= alpha;
		} else {
			c = texture(palArray, vec3(c.r * 0.9966, 0.5, float(palLayer)));
			if (mask == -1) {
				c.a = 1.0;
			}
		}
		if (hue != 0) {
			c.rgb = hue_shift(c.rgb,hue);			
		}
		if (neg) c.rgb = neg_base - c.rgb;
		c.rgb = mix(c.rgb, vec3((c.r + c.g + c.b) / 3.0), gray) + final_add;
		c *= final_mul;

		// Add a final tint (used for shadows); make sure the result has premultiplied alpha
		c.rgb = mix(c.rgb, tint.rgb * c.a, tint.a);

		FragColor = c;
	}
}
