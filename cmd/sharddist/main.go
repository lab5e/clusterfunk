package main

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
)

// This program uses shard map to build a series of images showing the
// distribution of shards, one color per node.

var nodeColors = []color.NRGBA{
	color.NRGBA{R: 255, G: 0, B: 0, A: 255},   // red
	color.NRGBA{R: 0, G: 255, B: 0, A: 255},   // green
	color.NRGBA{R: 0, G: 0, B: 255, A: 255},   // blue
	color.NRGBA{R: 255, G: 255, B: 0, A: 255}, // yellow
	color.NRGBA{R: 0, G: 255, B: 255, A: 255}, // cyan
	color.NRGBA{R: 255, G: 0, B: 255, A: 255}, // purple
}

func dumpImage(name string, sm sharding.ShardMap) {
	shards := sm.Shards()
	width := 128
	height := (len(shards) / width)
	if len(shards)%width > 0 {
		height++
	}
	colorMap := make(map[string]color.NRGBA)
	for i, node := range sm.NodeList() {
		colorMap[node] = nodeColors[i]
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	shardNo := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if shardNo < len(shards) {
				col := colorMap[shards[shardNo].NodeID()]
				img.Set(x, y, col)
			} else {
				img.Set(x, y, color.NRGBA{
					R: 255,
					G: 255,
					B: 255,
					A: 255,
				})
			}
			shardNo++
		}
	}
	f, err := os.Create(name)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err.Error())
	}
}

// Debugging: Make an image to visualise the distribution of shards across
// nodes.
func main() {
	sm := sharding.NewShardMap()

	const maxShards = 8192
	weights := make([]int, maxShards)
	for i := range weights {
		weights[i] = 1
	}
	sm.Init(maxShards)
	sm.UpdateNodes("A")
	dumpImage("a_01node.png", sm)
	sm.UpdateNodes("A", "B")
	dumpImage("a_02node.png", sm)
	sm.UpdateNodes("A", "B", "C")
	dumpImage("a_03node.png", sm)
	sm.UpdateNodes("A", "B", "C", "D")
	dumpImage("a_04node.png", sm)
	sm.UpdateNodes("A", "B", "C", "D", "E")
	dumpImage("a_05node.png", sm)

	sm.UpdateNodes("A", "B", "C", "D", "E", "F")
	dumpImage("a_06node.png", sm)

	sm.UpdateNodes("A", "B", "C", "D", "E")
	dumpImage("b_05node.png", sm)
	sm.UpdateNodes("A", "B", "C", "D")
	dumpImage("b_04node.png", sm)
	sm.UpdateNodes("A", "B", "C")
	dumpImage("b_03node.png", sm)
	sm.UpdateNodes("A", "B")
	dumpImage("b_02node.png", sm)
	sm.UpdateNodes("A")
	dumpImage("b_01node.png", sm)
}
