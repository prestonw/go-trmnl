//go:build inkplatetest

package main

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"time"

	"github.com/gesellix/go-trmnl/internal/plugins"
	"github.com/gesellix/go-trmnl/internal/plugins/weather"
	"github.com/gesellix/go-trmnl/internal/render"
)

func main() {
	fs := plugins.NewFontSet("", "", "", "", "")
	ctx := context.Background()

	in := plugins.RenderInput{
		Width:  800,
		Height: 600,
		Fonts:  fs,
		Now:    time.Now(),
		Settings: json.RawMessage(`{"location":"Dublin","theme":"inkplate","timezone":"Europe/Dublin"}`),
	}
	p := weather.New()
	model, err := p.DataModel(ctx, in)
	if err != nil {
		panic(err)
	}
	img, err := p.Render(ctx, in, model)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create("/tmp/inkplate-color.png")
	png.Encode(f, img)
	f.Close()

	res, err := render.NewRenderer("/tmp/inkplate-cache").Process(img, render.FloydSteinberg)
	if err != nil {
		panic(err)
	}
	bmp, _ := os.ReadFile("/tmp/inkplate-cache/" + res.BMPName)
	os.WriteFile("/tmp/inkplate-1bit.bmp", bmp, 0644)
	png1, _ := os.ReadFile("/tmp/inkplate-cache/" + res.PNGName)
	os.WriteFile("/tmp/inkplate-1bit.png", png1, 0644)
	println("OK", res.Hash)
}
