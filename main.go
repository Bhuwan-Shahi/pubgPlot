package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"strconv"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
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

// Function to draw a filled circle for player positions
func drawPlayerPoint(img *image.RGBA, x, y, radius int, col color.Color) {
	minX := max(0, x-radius)
	maxX := min(img.Bounds().Dx()-1, x+radius)
	minY := max(0, y-radius)
	maxY := min(img.Bounds().Dy()-1, y+radius)

	for i := minX; i <= maxX; i++ {
		for j := minY; j <= maxY; j++ {
			if (i-x)*(i-x)+(j-y)*(j-y) <= radius*radius {
				img.Set(i, j, col)
			}
		}
	}
}

// Function to draw text on the image
func addLabel(img *image.RGBA, x, y int, label string, col color.Color) {
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

	// Create a new drawer for the text
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}

	// Draw the text
	d.DrawString(label)
}

// Function to draw an ellipse with proper alpha blending
func drawEllipseOptimized(img *image.RGBA, centerX, centerY int, radiusX, radiusY float64, col color.Color) {
	radiusX2 := radiusX * radiusX
	radiusY2 := radiusY * radiusY
	minX := max(0, centerX-int(radiusX))
	maxX := min(img.Bounds().Dx()-1, centerX+int(radiusX))
	minY := max(0, centerY-int(radiusY))
	maxY := min(img.Bounds().Dy()-1, centerY+int(radiusY))

	rgba, ok := col.(color.RGBA)
	if !ok {
		// Convert to RGBA if it's not already
		r, g, b, a := col.RGBA()
		rgba = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}

	alphaF := float64(rgba.A) / 255.0

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			if (dx*dx)/radiusX2+(dy*dy)/radiusY2 <= 1 {
				// Proper alpha blending
				originalColor := img.At(x, y)
				or, og, ob, oa := originalColor.RGBA()

				nr := uint8((float64(or>>8)*(1-alphaF) + float64(rgba.R)*alphaF))
				ng := uint8((float64(og>>8)*(1-alphaF) + float64(rgba.G)*alphaF))
				nb := uint8((float64(ob>>8)*(1-alphaF) + float64(rgba.B)*alphaF))
				na := uint8(oa >> 8)

				img.Set(x, y, color.RGBA{nr, ng, nb, na})
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	// Command line flags
	mapFile := flag.String("map", "erangel.png", "Path to the map image")
	dataFile := flag.String("data", "message.json", "Path to the JSON data file")
	outputFile := flag.String("output", "mapDone.png", "Path for the output image")
	mapSize := flag.Float64("size", 800000.0, "Map size in game units (typically cm)")
	playerRadius := flag.Int("player-radius", 3, "Radius of player markers in pixels")
	playerColor := flag.String("player-color", "#FF0000", "Color of player markers in hex format")
	nameColor := flag.String("name-color", "#05ED0A", "Color of player names in hex format")
	zoneColor := flag.String("zone-color", "#00FF0080", "Color of safe zone in hex format with alpha")
	showNames := flag.Bool("show-names", true, "Show player names on the map")
	nameOffset := flag.Int("name-offset", 5, "Offset for player names in pixels")
	flag.Parse()

	// Parse player color
	playerCol, err := parseHexColor(*playerColor)
	if err != nil {
		fmt.Println("Error parsing player color:", err)
		return
	}

	// Parse name color
	nameCol, err := parseHexColor(*nameColor)
	if err != nil {
		fmt.Println("Error parsing name color:", err)
		return
	}

	// Parse zone color
	zoneCol, err := parseHexColor(*zoneColor)
	if err != nil {
		fmt.Println("Error parsing zone color:", err)
		return
	}

	// Load the map image
	file, err := os.Open(*mapFile)
	if err != nil {
		fmt.Println("Error opening map image:", err)
		return
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		fmt.Println("Error decoding map image:", err)
		return
	}

	// Convert the image to RGBA for drawing
	bounds := img.Bounds()
	rgbaImg := image.NewRGBA(bounds)
	draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)

	// Read and parse the JSON data
	data, err := ioutil.ReadFile(*dataFile)
	if err != nil {
		fmt.Println("Error reading JSON data:", err)
		return
	}

	var root RootData
	err = json.Unmarshal(data, &root)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	scaleX := float64(bounds.Dx()-1) / *mapSize
	scaleY := float64(bounds.Dy()-1) / *mapSize

	// Mutex for concurrent drawing
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Plot player locations concurrently
	for _, player := range root.InGameData.TotalPlayerList {
		wg.Add(1)
		go func(p Player) {
			defer wg.Done()

			// Convert game coordinates to pixel coordinates
			pixelX := int(p.Location.X * scaleX)
			pixelY := int(float64(bounds.Dy()-1) - p.Location.Y*scaleY)

			// Acquire lock when drawing to the shared image
			mutex.Lock()
			// Draw the player marker
			drawPlayerPoint(rgbaImg, pixelX, pixelY, *playerRadius, playerCol)

			// Add player name if enabled
			if *showNames && p.PlayerName != "" {
				// Position the name label slightly offset from the player marker
				labelX := pixelX + *nameOffset
				labelY := pixelY + *nameOffset
				addLabel(rgbaImg, labelX, labelY, p.PlayerName, nameCol)
			}
			mutex.Unlock()
		}(player)
	}

	wg.Wait()

	// Draw all safe zones
	for i, circle := range root.GameGlobalInfo.CircleArray {
		centerXFloat, centerYFloat, sizeFloat, err := circle.ToFloatValues()
		if err != nil {
			fmt.Printf("Error converting circle %d data to floats: %v\n", i, err)
			continue
		}

		centerX := int(centerXFloat * scaleX)
		centerY := int(float64(bounds.Dy()-1) - centerYFloat*scaleY)
		radiusX := sizeFloat * scaleX
		radiusY := sizeFloat * scaleY

		thisZoneCol := zoneCol
		if i > 0 {
			r, g, b, a := zoneCol.RGBA()
			thisZoneCol = color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a>>8) / uint8(i+1),
			}
		}

		drawEllipseOptimized(rgbaImg, centerX, centerY, radiusX, radiusY, thisZoneCol)
	}

	addLabel(rgbaImg, 10, 20, "Players: "+strconv.Itoa(len(root.InGameData.TotalPlayerList)), nameCol)
	addLabel(rgbaImg, 10, 40, "Safe Zones: "+strconv.Itoa(len(root.GameGlobalInfo.CircleArray)), nameCol)

	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer output.Close()

	err = png.Encode(output, rgbaImg)
	if err != nil {
		fmt.Println("Error encoding output image:", err)
		return
	}

	fmt.Printf("Map with player locations, names, and safe zones has been saved as %s\n", *outputFile)
}

func parseHexColor(hex string) (color.RGBA, error) {
	if len(hex) != 7 && len(hex) != 9 {
		return color.RGBA{}, fmt.Errorf("invalid hex color format: %s", hex)
	}

	hex = hex[1:]

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}

	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}

	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}

	a := uint64(255)

	if len(hex) == 8 {
		a, err = strconv.ParseUint(hex[6:8], 16, 8)
		if err != nil {
			return color.RGBA{}, err
		}
	}

	return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
}
