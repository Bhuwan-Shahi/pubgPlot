package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Structs to parse the JSON data
type Location struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Player struct {
	UID        int64    `json:"uId"`
	PlayerName string   `json:"playerName"`
	Location   Location `json:"location"`
}

type InGameData struct {
	TotalPlayerList []Player `json:"TotalPlayerList"`
}

type CircleData struct {
	X    string `json:"X"`
	Y    string `json:"Y"`
	Size string `json:"Size"`
}

func (c *CircleData) ToFloatValues() (x, y, size float64, err error) {
	x, err = strconv.ParseFloat(c.X, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse X: %v", err)
	}
	y, err = strconv.ParseFloat(c.Y, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse Y: %v", err)
	}
	size, err = strconv.ParseFloat(c.Size, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse Size: %v", err)
	}
	return x, y, size, nil
}

type GameGlobalInfo struct {
	CircleArray []CircleData `json:"CircleArray"`
}

type RootData struct {
	InGameData     InGameData     `json:"inGameData"`
	GameGlobalInfo GameGlobalInfo `json:"gameGlobalInfo"`
}

func drawPlayerPoint(img *image.RGBA, centerX, centerY int, radius int, col color.Color) {
	for x := centerX - radius; x <= centerX+radius; x++ {
		for y := centerY - radius; y <= centerY+radius; y++ {
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			if dx*dx+dy*dy <= float64(radius*radius) {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, col)
				}
			}
		}
	}
}

func drawEllipse(img *image.RGBA, centerX, centerY int, radiusX, radiusY float64, col color.Color) {
	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			if (dx*dx)/(radiusX*radiusX)+(dy*dy)/(radiusY*radiusY) <= 1 {
				currentColor := img.RGBA64At(x, y)
				newColor := col.(color.RGBA)
				alpha := float64(newColor.A) / 255.0
				r := uint16(float64(currentColor.R)*(1-alpha) + float64(newColor.R)*alpha*255)
				g := uint16(float64(currentColor.G)*(1-alpha) + float64(newColor.G)*alpha*255)
				b := uint16(float64(currentColor.B)*(1-alpha) + float64(newColor.B)*alpha*255)
				a := currentColor.A
				img.Set(x, y, color.RGBA64{R: r, G: g, B: b, A: a})
			}
		}
	}
}

func loadFont() (*opentype.Font, error) {
	fontBytes, err := ioutil.ReadFile("LiberationSans-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to read font file: %v", err)
	}

	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %v", err)
	}
	return f, nil
}

func main() {
	file, err := os.Open("erangel.png")
	if err != nil {
		fmt.Println("Error opening image:", err)
		return
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		fmt.Println("Error decoding image:", err)
		return
	}

	bounds := img.Bounds()
	rgbaImg := image.NewRGBA(bounds)
	draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)

	fnt, err := loadFont()
	if err != nil {
		fmt.Println("Error loading font:", err)
		return
	}

	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    14,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		fmt.Println("Error creating font face:", err)
		return
	}
	defer face.Close()

	data, err := ioutil.ReadFile("message.json")
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	var root RootData
	err = json.Unmarshal(data, &root)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Step 4: Map dimensions and conversion
	const mapSizeCm = 800000.0
	imgWidth := 1080
	imgHeight := 1080

	// Step 5: Plot player locations and names
	playerColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	textColor := color.RGBA{R: 0, G: 243, B: 6, A: 255}
	for _, player := range root.InGameData.TotalPlayerList {
		// Convert game coordinates to pixel coordinates
		pixelX := int((player.Location.X / mapSizeCm) * float64(imgWidth-1))
		pixelY := int((player.Location.Y / mapSizeCm) * float64(imgHeight-1))

		drawPlayerPoint(rgbaImg, pixelX, pixelY, 4, playerColor)

		d := &font.Drawer{
			Dst:  rgbaImg,
			Src:  image.NewUniform(textColor),
			Face: face,
			Dot:  fixed.Point26_6{X: fixed.I(pixelX + 10), Y: fixed.I(pixelY)}, // Offset text 10 pixels to the right
		}
		d.DrawString(player.PlayerName)
	}

	if len(root.GameGlobalInfo.CircleArray) > 0 {
		circle := root.GameGlobalInfo.CircleArray[0]
		centerXFloat, centerYFloat, sizeFloat, err := circle.ToFloatValues()
		if err != nil {
			fmt.Println("Error converting circle data to floats:", err)
			return
		}

		centerX := int((centerXFloat / mapSizeCm) * float64(imgWidth-1))
		centerY := int((centerYFloat / mapSizeCm) * float64(imgHeight-1))
		radiusX := (sizeFloat / mapSizeCm) * float64(imgWidth-1)
		radiusY := (sizeFloat / mapSizeCm) * float64(imgHeight-1)

		circleColor := color.RGBA{R: 0, G: 255, B: 0, A: 128} // Green with transparency
		drawEllipse(rgbaImg, centerX, centerY, radiusX, radiusY, circleColor)
	}

	// Step 7: Save the modified image
	outputFile, err := os.Create("doneMap.png")
	if err != nil {
		fmt.Println("Error creating doneMap file:", err)
		return
	}
	defer outputFile.Close()

	err = png.Encode(outputFile, rgbaImg)
	if err != nil {
		fmt.Println("Error encoding doneMap :", err)
		return
	}

	fmt.Println("Map with player locations, names, and safe zone has been saved as output.png")
}
