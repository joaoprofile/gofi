// Package clipboard reads a picture for the chat: the one the system
// clipboard holds, or an image file the person pointed at.
//
// There is no portable clipboard for pictures, so each system is asked the
// way it answers: PowerShell on Windows and under WSL — where the clipboard
// is Windows's —, wl-paste or xclip on Linux, pngpaste or AppleScript on
// macOS. Whatever comes is fitted to what a model reads: a picture past
// MaxEdge is scaled down before it is sent, since the tokens it costs grow
// with its size and the model would scale it down anyway.
package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // registered so a pasted GIF is measured
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrNoImage is returned when the clipboard holds no picture.
var ErrNoImage = errors.New("no image in the clipboard")

// MaxEdge is the longest side a picture is sent with, in pixels. A larger one
// is scaled down to it.
const MaxEdge = 1568

// maxBytes is the size a picture is sent under: past it a PNG goes as JPEG.
const maxBytes = 3_500_000

// Picture is an image ready to go with a turn.
type Picture struct {
	MediaType     string
	Data          []byte
	Width, Height int
}

// Read is the picture in the clipboard, fitted. ErrNoImage when it holds
// none, or when this system has no way to ask.
func Read(ctx context.Context) (Picture, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	raw, err := read(ctx)
	if err != nil {
		return Picture{}, err
	}
	return Fit(raw)
}

// File is an image file, fitted. ok is false when the path does not name an
// image the engine reads.
func File(path string) (pic Picture, ok bool, err error) {
	if !IsImagePath(path) {
		return Picture{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Picture{}, true, err
	}
	pic, err = Fit(raw)
	return pic, true, err
}

// IsImagePath reports whether a path names an image by its extension.
func IsImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}

// Fit makes raw image bytes ready to send: measured, scaled down when a side
// passes MaxEdge, and under the size the engine takes. What cannot be decoded
// here but is an image by its signature — WebP — goes as it is.
func Fit(raw []byte) (Picture, error) {
	media := sniff(raw)
	if media == "" {
		return Picture{}, ErrNoImage
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		if media == "image/webp" && len(raw) <= maxBytes {
			return Picture{MediaType: media, Data: raw}, nil
		}
		return Picture{}, fmt.Errorf("unreadable image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= MaxEdge && h <= MaxEdge && len(raw) <= maxBytes && media != "image/gif" {
		return Picture{MediaType: media, Data: raw, Width: w, Height: h}, nil
	}
	if long := max(w, h); long > MaxEdge {
		w, h = max(w*MaxEdge/long, 1), max(h*MaxEdge/long, 1)
		img = shrink(img, w, h)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return Picture{}, err
	}
	if out.Len() <= maxBytes {
		return Picture{MediaType: "image/png", Data: out.Bytes(), Width: w, Height: h}, nil
	}
	out.Reset()
	if err := jpeg.Encode(&out, opaque(img), &jpeg.Options{Quality: 85}); err != nil {
		return Picture{}, err
	}
	return Picture{MediaType: "image/jpeg", Data: out.Bytes(), Width: w, Height: h}, nil
}

// sniff is the media type an image's first bytes say it is.
func sniff(raw []byte) string {
	switch {
	case bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(raw, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case bytes.HasPrefix(raw, []byte("GIF87a")), bytes.HasPrefix(raw, []byte("GIF89a")):
		return "image/gif"
	case len(raw) > 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

// shrink scales an image down to w by h, each pixel the average of the ones
// it stands for — what keeps small text readable where taking one pixel in
// every few would not.
func shrink(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	rgba, ok := src.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(b)
		draw.Draw(rgba, b, src, b.Min, draw.Src)
	}
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := range w {
			x0, x1 := x*sw/w, (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint32
			for yy := y0; yy < y1; yy++ {
				i := rgba.PixOffset(b.Min.X+x0, b.Min.Y+yy)
				for xx := x0; xx < x1; xx++ {
					r += uint32(rgba.Pix[i])
					g += uint32(rgba.Pix[i+1])
					bl += uint32(rgba.Pix[i+2])
					a += uint32(rgba.Pix[i+3])
					i += 4
					n++
				}
			}
			o := dst.PixOffset(x, y)
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(bl/n), uint8(a/n)
		}
	}
	return dst
}

// opaque lays an image over white: JPEG has no transparency.
func opaque(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	return dst
}

// read asks this system for the clipboard's picture, as the bytes of an
// image file.
func read(ctx context.Context) ([]byte, error) {
	switch {
	case runtime.GOOS == "windows" || wsl():
		return fromPowerShell(ctx)
	case runtime.GOOS == "darwin":
		return fromMac(ctx)
	}
	return fromLinux(ctx)
}

// wsl reports a Linux running under Windows: the clipboard is Windows's.
func wsl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	v, _ := os.ReadFile("/proc/version")
	return strings.Contains(strings.ToLower(string(v)), "microsoft")
}

// psScript writes the clipboard's picture as base64 PNG, or the path of a
// copied image file after "FILE:", or nothing.
const psScript = `Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; ` +
	`$i=[System.Windows.Forms.Clipboard]::GetImage(); ` +
	`if($i -ne $null){$m=New-Object System.IO.MemoryStream; $i.Save($m,[System.Drawing.Imaging.ImageFormat]::Png); [Console]::Out.Write([Convert]::ToBase64String($m.ToArray()))} ` +
	`else{$f=[System.Windows.Forms.Clipboard]::GetFileDropList(); if($f.Count -gt 0){[Console]::Out.Write('FILE:'+$f[0])}}`

func fromPowerShell(ctx context.Context) ([]byte, error) {
	exe := "powershell.exe"
	if _, err := exec.LookPath(exe); err != nil {
		// Under WSL with the Windows PATH left out, it is still where it is.
		exe = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
		if _, err := os.Stat(exe); err != nil {
			return nil, ErrNoImage
		}
	}
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-STA", "-Command", psScript).Output()
	if err != nil {
		return nil, fmt.Errorf("read the clipboard: %w", err)
	}
	text := strings.TrimSpace(string(out))
	switch {
	case text == "":
		return nil, ErrNoImage
	case strings.HasPrefix(text, "FILE:"):
		path := strings.TrimPrefix(text, "FILE:")
		if !IsImagePath(path) {
			return nil, ErrNoImage
		}
		if runtime.GOOS != "windows" {
			unix, err := exec.CommandContext(ctx, "wslpath", "-u", path).Output()
			if err != nil {
				return nil, ErrNoImage
			}
			path = strings.TrimSpace(string(unix))
		}
		return os.ReadFile(path)
	}
	return base64.StdEncoding.DecodeString(text)
}

func fromLinux(ctx context.Context) ([]byte, error) {
	tries := [][]string{
		{"wl-paste", "--no-newline", "--type", "image/png"},
		{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"},
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		tries[0], tries[1] = tries[1], tries[0]
	}
	for _, t := range tries {
		if _, err := exec.LookPath(t[0]); err != nil {
			continue
		}
		if out, err := exec.CommandContext(ctx, t[0], t[1:]...).Output(); err == nil && sniff(out) != "" {
			return out, nil
		}
	}
	return nil, ErrNoImage
}

func fromMac(ctx context.Context) ([]byte, error) {
	if _, err := exec.LookPath("pngpaste"); err == nil {
		if out, err := exec.CommandContext(ctx, "pngpaste", "-").Output(); err == nil && sniff(out) != "" {
			return out, nil
		}
	}
	f, err := os.CreateTemp("", "gofi-clip-*.png")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	script := fmt.Sprintf(`set f to open for access POSIX file %q with write permission
try
	write (the clipboard as «class PNGf») to f
end try
close access f`, path)
	if err := exec.CommandContext(ctx, "osascript", "-e", script).Run(); err != nil {
		return nil, ErrNoImage
	}
	out, err := os.ReadFile(path)
	if err != nil || sniff(out) == "" {
		return nil, ErrNoImage
	}
	return out, nil
}
