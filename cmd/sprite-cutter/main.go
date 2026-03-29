package main

import (
	"flag"
	"log"
	"strconv"
	"strings"

	"sprite-cutter/internal/cutter" // путь должен совпадать с module в go.mod
)

func main() {
	input := flag.String("input", "", "путь к входному изображению")
	output := flag.String("output", "images/output", "папка для сохранения")
	bg := flag.String("bg", "0,0", "координаты фона x,y")
	threshold := flag.Uint("threshold", cutter.DefaultThreshold, "порог схожести цвета (0-765)")
	vert := flag.Int("vert", 20, "вертикальный порог группировки (пиксели)")
	horiz := flag.Int("horiz", 50, "горизонтальный порог группировки (пиксели)")
	flag.Parse()

	if *input == "" {
		log.Fatal("не указан -input")
	}

	parts := strings.Split(*bg, ",")
	if len(parts) != 2 {
		log.Fatal("bg должен быть в формате x,y")
	}
	x, _ := strconv.Atoi(parts[0])
	y, _ := strconv.Atoi(parts[1])

	err := cutter.Process(*input, *output, x, y, uint32(*threshold), *vert, *horiz)
	if err != nil {
		log.Fatal(err)
	}
}
