package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFitKeepsASmallPictureAsItIs(t *testing.T) {
	raw := pngOf(t, 320, 200)
	pic, err := Fit(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pic.MediaType != "image/png" || pic.Width != 320 || pic.Height != 200 || !bytes.Equal(pic.Data, raw) {
		t.Errorf("a picture that fits must go untouched: %s %dx%d", pic.MediaType, pic.Width, pic.Height)
	}
}

func TestFitScalesALargePictureDown(t *testing.T) {
	pic, err := Fit(pngOf(t, 3136, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if pic.Width != MaxEdge || pic.Height != 500 {
		t.Errorf("scaled to %dx%d, want %dx500", pic.Width, pic.Height, MaxEdge)
	}
	got, err := png.Decode(bytes.NewReader(pic.Data))
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != MaxEdge || b.Dy() != 500 {
		t.Errorf("the bytes say %dx%d", b.Dx(), b.Dy())
	}
}

func TestFitRefusesWhatIsNotAPicture(t *testing.T) {
	if _, err := Fit([]byte("hello")); err != ErrNoImage {
		t.Errorf("err = %v, want ErrNoImage", err)
	}
}

func TestFileReadsOnlyImages(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "shot.PNG")
	if err := os.WriteFile(shot, pngOf(t, 10, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	if pic, ok, err := File(shot); !ok || err != nil || pic.Width != 10 {
		t.Errorf("File(shot) = %+v %v %v", pic.Width, ok, err)
	}
	if _, ok, _ := File(filepath.Join(dir, "notes.txt")); ok {
		t.Error("a text file is not an image")
	}
	if _, ok, err := File(filepath.Join(dir, "gone.png")); !ok || err == nil {
		t.Error("a missing image is an image that could not be read")
	}
}
