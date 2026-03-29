package main

import (
	"flag"
	"log"

	"sprite-cutter/internal/gifmaker"
)

func main() {
	inputDir := flag.String("input", "images/output", "папка с PNG-файлами sprite_XX_YY.png")
	outputDir := flag.String("output", "images/gifs", "папка для сохранения GIF")
	delay := flag.Int("delay", 10, "задержка между кадрами в сотых долях секунды (10 = 0.1 сек)")
	loop := flag.Int("loop", 0, "количество повторов: 0 = бесконечно, -1 = один раз")
	flag.Parse()

	cfg := gifmaker.Config{
		InputDir:  *inputDir,
		OutputDir: *outputDir,
		Delay:     *delay,
		LoopCount: *loop,
	}

	if err := gifmaker.ProcessAllGroups(cfg); err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
	log.Println("GIF-анимации успешно созданы!")
}
