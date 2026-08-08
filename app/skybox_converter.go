package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qcountel/zUtility/pkg/embeddable"
)

const (
	skyVertexHLSL = `#include "ShaderConstants.fxh"

struct VS_Input
{
    float3 position : POSITION;
    float2 uv : TEXCOORD_0;
#ifdef INSTANCEDSTEREO
    uint instanceID : SV_InstanceID;
#endif
};

struct PS_Input
{
    float4 position : SV_Position;
    float2 uv : TEXCOORD_0_FB_MSAA;
    float3 v_pos : TEXCOORD_1;
#ifdef INSTANCEDSTEREO
    uint instanceID : SV_InstanceID;
#endif
};

void main( in VS_Input VSInput, out PS_Input PSInput )
{
    PSInput.v_pos = VSInput.position;
    PSInput.uv = VSInput.uv;

#ifdef INSTANCEDSTEREO
    int i = VSInput.instanceID;
    PSInput.position = mul( WORLDVIEWPROJ_STEREO[i], float4( VSInput.position, 1 ) );
    PSInput.instanceID = i;
#else
    PSInput.position = mul(WORLDVIEWPROJ, float4(VSInput.position, 1));
#endif
}
`

	skyFragmentHLSL = `#include "ShaderConstants.fxh"

struct PS_Input
{
    float4 position : SV_Position;
    float2 uv : TEXCOORD_0_FB_MSAA;
    float3 v_pos : TEXCOORD_1;
};

struct PS_Output
{
    float4 color : SV_Target;
};

void main( in PS_Input PSInput, out PS_Output PSOutput )
{
    float3 dir = normalize(PSInput.v_pos);
    float3 a = abs(dir);
    float ma = max(a.x, max(a.y, a.z));

    float u = 0.0;
    float v = 0.0;
    float faceIndex = 0.0;

    if (ma == a.z) {
        if (dir.z > 0.0) {
            u = -dir.x / a.z;
            v = dir.y / a.z;
            faceIndex = 0.0;
        } else {
            u = dir.x / a.z;
            v = dir.y / a.z;
            faceIndex = 2.0;
        }
    } else if (ma == a.x) {
        if (dir.x > 0.0) {
            u = dir.z / a.x;
            v = dir.y / a.x;
            faceIndex = 3.0;
        } else {
            u = -dir.z / a.x;
            v = dir.y / a.x;
            faceIndex = 1.0;
        }
    } else {
        if (dir.y > 0.0) {
            u = -dir.x / a.y;
            v = -dir.z / a.y;
            faceIndex = 5.0;
        } else {
            u = -dir.x / a.y;
            v = dir.z / a.y;
            faceIndex = 4.0;
        }
    }

    u = u * 0.5 + 0.5;
    v = v * 0.5 + 0.5;
    u = (u + faceIndex) / 6.0;

    float4 diffuse = TEXTURE_0.Sample(TextureSampler0, float2(u, v));
    PSOutput.color = CURRENT_COLOR * diffuse;
}
`

	skyMaterialJSON = `{
  "end_sky": {
    "states": ["DisableDepthWrite", "DisableAlphaWrite"],
    "msaaSupport": "MSAA",
    "vertexShader": "shaders/end_sky_cubemap.vertex",
    "vrGeometryShader": "shaders/uv.geometry",
    "fragmentShader": "shaders/end_sky_cubemap.fragment",
    "vertexFields": [
      { "field": "Position" },
      { "field": "UV0" }
    ],
    "samplerStates": [
      { "samplerIndex": 0, "textureWrap": "Clamp" }
    ]
  }
}`
)

// ConvertAndImportSkyboxPack конвертирует .mcpack кубмапа в формат Win10 Sky
// и с уведомлением об этапах процесса через onProgress callback.
func ConvertAndImportSkyboxPack(inputPath string, clonedOnly bool, onProgress func(step string, percent float64)) (string, error) {
	report := func(step string, p float64) {
		if onProgress != nil {
			onProgress(step, p)
		}
		time.Sleep(40 * time.Millisecond)
	}

	report("Открытие архива ресурпака...", 0.10)

	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return "", fmt.Errorf("ошибка открытия архива: %w", err)
	}
	defer r.Close()

	report("Извлечение 6 граней панорамы (cubemap_0..5.png)...", 0.25)

	var faces [6]image.Image
	found := 0
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		for i := 0; i < 6; i++ {
			suffix := fmt.Sprintf("cubemap_%d.png", i)
			if strings.HasSuffix(name, suffix) && faces[i] == nil {
				rc, err := f.Open()
				if err != nil {
					return "", err
				}
				img, err := png.Decode(rc)
				rc.Close()
				if err != nil {
					return "", fmt.Errorf("ошибка декодирования %s: %w", f.Name, err)
				}
				faces[i] = img
				found++
			}
		}
	}

	if found < 6 {
		return "", fmt.Errorf("в пакете найдено только %d из 6 граней кубмапа (cubemap_0..5.png)", found)
	}

	report("Сборка панорамного 6×1 текстурного стрипа...", 0.50)

	fW := faces[0].Bounds().Dx()
	fH := faces[0].Bounds().Dy()

	strip := image.NewRGBA(image.Rect(0, 0, fW*6, fH))
	for i, face := range faces {
		var src image.Image = face
		if face.Bounds().Dx() != fW || face.Bounds().Dy() != fH {
			src = scaleNearest(face, fW, fH)
		}
		dst := image.Rect(i*fW, 0, (i+1)*fW, fH)
		draw.Draw(strip, dst, src, src.Bounds().Min, draw.Src)
	}

	var stripBuf bytes.Buffer
	if err := png.Encode(&stripBuf, strip); err != nil {
		return "", fmt.Errorf("ошибка кодирования PNG стрипа: %w", err)
	}

	report("Создание невидимой текстуры облаков (clouds.png)...", 0.65)

	transparentCloud := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var cloudBuf bytes.Buffer
	_ = png.Encode(&cloudBuf, transparentCloud)

	report("Компиляция HLSL-шейдеров и генерация манифеста...", 0.80)

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	folderName := sanitizeName(baseName + "_Win10Sky")

	manifest := map[string]interface{}{
		"format_version": 1,
		"header": map[string]interface{}{
			"description": fmt.Sprintf("Custom Sky - converted from %s", baseName),
			"name":        fmt.Sprintf("%s (Win10 Sky)", baseName),
			"uuid":        generateSkyUUID(baseName + "-header"),
			"version":     []int{1, 0, 0},
		},
		"modules": []map[string]interface{}{
			{
				"description": "Resources",
				"type":        "resources",
				"uuid":        generateSkyUUID(baseName + "-module"),
				"version":     []int{1, 0, 0},
			},
		},
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")

	report("Импорт ресурпака в директорию Minecraft...", 0.92)

	dirs := getInstalledPacksDirs(clonedOnly)
	if len(dirs) == 0 {
		return "", fmt.Errorf("папка ресурпаков Minecraft не найдена")
	}

	for _, packDir := range dirs {
		targetFolder := filepath.Join(packDir, folderName)
		_ = os.MkdirAll(targetFolder, 0755)

		writeFile := func(relPath string, data []byte) error {
			full := filepath.Join(targetFolder, relPath)
			_ = os.MkdirAll(filepath.Dir(full), 0755)
			return os.WriteFile(full, data, 0644)
		}

		_ = writeFile("manifest.json", manifestJSON)
		_ = writeFile("materials/sky.material", []byte(skyMaterialJSON))
		_ = writeFile("shaders/dx11/end_sky_cubemap.vertex.hlsl", []byte(skyVertexHLSL))
		_ = writeFile("shaders/dx11/end_sky_cubemap.fragment.hlsl", []byte(skyFragmentHLSL))
		_ = writeFile("textures/environment/end_sky.png", stripBuf.Bytes())
		_ = writeFile("textures/environment/clouds.png", cloudBuf.Bytes())

		if iconBytes := embeddable.IconBytes(); len(iconBytes) > 0 {
			_ = writeFile("pack_icon.png", iconBytes)
		}
	}

	report("Успешно импортировано!", 1.00)

	return fmt.Sprintf("%s (Win10 Sky)", baseName), nil
}

func scaleNearest(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	for y := 0; y < h; y++ {
		sy := y * sh / h
		for x := 0; x < w; x++ {
			sx := x * sw / w
			dst.Set(x, y, src.At(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy))
		}
	}
	return dst
}

func generateSkyUUID(seed string) string {
	h := uint64(14695981039346656037)
	for _, c := range []byte(seed) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	h2 := h ^ 0xdeadbeef12345678
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(h>>32),
		uint16(h>>16),
		uint16(h&0xffff)|0x4000,
		uint16(h2>>48)|0x8000,
		h2&0xffffffffffff,
	)
}
