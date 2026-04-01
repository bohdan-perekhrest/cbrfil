package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
)

const ImageMaxHeight int = 65535

type Chunk struct {
	Height, Width, EndIdx int
}

func exitWithMessage(msg string, err error) {
	fmt.Println("cbrfil:", msg, " | ", err)
	os.Exit(1)
}

func extractImages(path string) ([]image.Image, error) {
	archive, err := zip.OpenReader(path)
	if err != nil { exitWithMessage("Failed during opening an archive", err) }
	defer archive.Close()

	var images []image.Image
	for _, file := range archive.File {
		img, err := decodeFile(file)
		if err != nil { return nil, err }
		images = append(images, img)
	}
	return images, nil
}

func decodeFile(file *zip.File) (image.Image, error) {
	rc, err := file.Open()
	if err != nil { return nil, err }
	defer rc.Close()

	if err != nil { return nil, err }
	m, _, err := image.Decode(rc)
	return m, err
}

func calculateDimensions(images []image.Image) []Chunk {
	var result []Chunk
	var height int
	var width int

	for i, img := range images {
		b := img.Bounds()
		if height+b.Dy() > ImageMaxHeight {
			result = append(result, Chunk{Height: height, Width: width, EndIdx : i})
			height, width = 0, 0
		}
		height += b.Dy()
		if b.Dx() > width { width = b.Dx() }
	}

	if height != 0 {
		result = append(result, Chunk{Height: height, Width: width, EndIdx : len(images)})
	}

	return result
}

func convertImages(images []image.Image) []image.Image {
	var result []image.Image
	pages := calculateDimensions(images)
	idx := 0

	for _, chunk := range pages {
		canvas := image.NewRGBA(image.Rect(0, 0, chunk.Width, chunk.Height))
		offset := 0

		for _, img := range images[idx:chunk.EndIdx] {
			r := image.Rect(0, offset, img.Bounds().Dx(), offset+img.Bounds().Dy())
			draw.Draw(canvas, r, img, image.Point{}, draw.Src)
			offset += img.Bounds().Dy()
		}
		idx = chunk.EndIdx

		result = append(result, canvas)
	}

	return result
}

func createNewCBR(images []image.Image, path string) error {
	tmpPath := path+".tmp"
	out, err := os.Create(tmpPath)
	if err != nil { return err }
	zw := zip.NewWriter(out)
	defer out.Close()
	defer zw.Close()

	for i, img := range images {
		w, err := zw.Create(fmt.Sprintf("page_%03d.jpg", i))
		if err != nil { return err }
		err = jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
		if err != nil { return err }
	}

	os.Remove(path)
	err = os.Rename(tmpPath, path)
	if err != nil { return err }

	return nil
}

func processArchive(path string) {
	if strings.HasSuffix(path, ".cbr") || strings.HasSuffix(path, ".cbz") {
		images, err := extractImages(path)
		if err != nil { exitWithMessage("Failed during extracting images", err) }

		newImages := convertImages(images)

		err = createNewCBR(newImages, path)
		if err != nil { exitWithMessage("Failed during creating new cbr", err) }

		fmt.Printf("cbrfil: File %s was updated\n", path)
	}
}

func main() {
	if len(os.Args) < 2 { exitWithMessage("Usage: cbrfil [file|dir...]", nil) }

	for _, path := range os.Args[1:] {
		stat, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) { exitWithMessage("File does not exist", err) }
		if stat.IsDir() {
			files, err := os.ReadDir(path)
			if err != nil { exitWithMessage("Failed during reading a dir", err) }

			for _, file := range files {
				if file.IsDir() { continue }
				processArchive(filepath.Join(path, file.Name()))
			}
		} else {
			processArchive(path)
		}
	}
}
