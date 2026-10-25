package sipom

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestShrinkJPEG(t *testing.T) {
	var src bytes.Buffer
	if err := png.Encode(&src, image.NewRGBA(image.Rect(0, 0, 3000, 1500))); err != nil {
		t.Fatal(err)
	}
	out, err := ShrinkJPEG(src.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("saída não é JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1920 || b.Dy() != 960 {
		t.Errorf("tamanho: %dx%d, quer 1920x960", b.Dx(), b.Dy())
	}
	if _, err := ShrinkJPEG([]byte("não é imagem")); err == nil {
		t.Errorf("conteúdo inválido deveria dar erro")
	}
}
