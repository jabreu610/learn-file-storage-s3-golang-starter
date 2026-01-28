package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

const SixteenByNine = "16:9"
const NineBySixteen = "9:16"

const threshold = 0.01

var ratios = map[string]float64{
	NineBySixteen: 9.0 / 16.0, // 0.5625
	SixteenByNine: 16.0 / 9.0, // 1.778
}

type videoMeta struct {
	Streams []videoDimensions `json:"streams"`
}

type videoDimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	out := bytes.Buffer{}
	cmd.Stdout = &out
	fmt.Println(cmd.Args)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	var dimensions videoMeta
	if err := json.Unmarshal(out.Bytes(), &dimensions); err != nil {
		return "", err
	}
	width := dimensions.Streams[0].Width
	height := dimensions.Streams[0].Height
	r := float64(width) / float64(height)
	for ratio, val := range ratios {
		if math.Abs(r-val) <= threshold {
			return ratio, nil
		}
	}
	return "other", nil
}
