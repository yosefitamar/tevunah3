package sipom

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // decodifica PNG
)

// Medidas da foto reduzida para o envio: lado maior e qualidade JPEG — o
// bastante para o SIPOM, que regrava tudo em JPG.
const (
	shrinkMaxSide = 1920
	shrinkQuality = 82
)

// ShrinkJPEG regrava a imagem (JPEG ou PNG) em JPEG com no máximo
// shrinkMaxSide no lado maior. Reduz pela média de cada bloco de pixels
// (filtro de caixa), o que basta para fotos.
func ShrinkJPEG(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if side := max(w, h); side > shrinkMaxSide {
		nw, nh := w*shrinkMaxSide/side, h*shrinkMaxSide/side
		src = boxResize(src, max(nw, 1), max(nh, 1))
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: shrinkQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func boxResize(src image.Image, nw, nh int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, max((y+1)*h/nh, y*h/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, max((x+1)*w/nw, x*w/nw+1)
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, _ := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r, g, bl, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), n+1
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 0xff})
		}
	}
	return dst
}
