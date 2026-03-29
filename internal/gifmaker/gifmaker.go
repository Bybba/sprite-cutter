package gifmaker

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	InputDir  string
	OutputDir string
	Delay     int
	LoopCount int
}

// buildGlobalPalette собирает уникальные цвета и определяет прозрачный индекс
func buildGlobalPalette(images []image.Image) (color.Palette, int) {
	colorSet := make(map[color.Color]struct{})
	for _, img := range images {
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := img.At(x, y)
				// Игнорируем полностью прозрачные для палитры? Нет, но потом назначим им прозрачный индекс.
				colorSet[c] = struct{}{}
			}
		}
	}
	palette := make(color.Palette, 0, len(colorSet))
	for c := range colorSet {
		palette = append(palette, c)
	}
	if len(palette) > 256 {
		palette = palette[:256]
	}
	// Определяем прозрачный цвет (обычно цвет с альфа=0)
	transparent := color.RGBA{0, 0, 0, 0}
	transparentIndex := -1
	for i, c := range palette {
		// Если цвет полностью прозрачный или альфа-канал отсутствует? В RGBA можно сравнить.
		if rgba, ok := color.RGBAModel.Convert(c).(color.RGBA); ok && rgba.A == 0 {
			transparentIndex = i
			break
		}
	}
	// Если прозрачного цвета нет, добавим его
	if transparentIndex == -1 && len(palette) < 256 {
		palette = append(palette, transparent)
		transparentIndex = len(palette) - 1
	}
	return palette, transparentIndex
}

// centerImageInCanvas вписывает изображение в холст, устанавливая прозрачный фон
func centerImageInCanvas(img image.Image, targetWidth, targetHeight int, palette color.Palette, transparentIndex int) *image.Paletted {
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	canvas := image.NewPaletted(image.Rect(0, 0, targetWidth, targetHeight), palette)
	// Заполняем холст прозрачным цветом (устанавливаем все пиксели в transparentIndex)
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			canvas.SetColorIndex(x, y, uint8(transparentIndex))
		}
	}
	offsetX := (targetWidth - srcW) / 2
	offsetY := (targetHeight - srcH) / 2
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			sx := srcBounds.Min.X + x
			sy := srcBounds.Min.Y + y
			c := img.At(sx, sy)
			// Если цвет прозрачный, не рисуем (оставляем прозрачный)
			if rgba, ok := color.RGBAModel.Convert(c).(color.RGBA); ok && rgba.A == 0 {
				continue
			}
			canvas.Set(offsetX+x, offsetY+y, c)
		}
	}
	return canvas
}

func ProcessAllGroups(cfg Config) error {
	entries, err := os.ReadDir(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать папку %s: %w", cfg.InputDir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "sprite_") && strings.HasSuffix(name, ".png") {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("в папке %s нет файлов вида sprite_XX_YY.png", cfg.InputDir)
	}
	groups := make(map[int][]string)
	for _, f := range files {
		parts := strings.Split(strings.TrimSuffix(f, ".png"), "_")
		if len(parts) != 3 {
			continue
		}
		groupNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		groups[groupNum] = append(groups[groupNum], f)
	}
	fmt.Printf("Найдено групп: %d\n", len(groups))
	for gn, fs := range groups {
		fmt.Printf("Группа %d: %d файлов\n", gn, len(fs))
	}
	if err := os.MkdirAll(cfg.OutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("не удалось создать выходную папку: %w", err)
	}
	for groupNum, fileNames := range groups {
		sort.Slice(fileNames, func(i, j int) bool {
			return extractFrameNum(fileNames[i]) < extractFrameNum(fileNames[j])
		})
		var images []image.Image
		maxWidth, maxHeight := 0, 0
		for _, fileName := range fileNames {
			path := filepath.Join(cfg.InputDir, fileName)
			f, err := os.Open(path)
			if err != nil {
				fmt.Printf("  Группа %d: ошибка открытия %s: %v\n", groupNum, fileName, err)
				continue
			}
			img, err := png.Decode(f)
			f.Close()
			if err != nil {
				fmt.Printf("  Группа %d: ошибка декодирования %s: %v\n", groupNum, fileName, err)
				continue
			}
			bounds := img.Bounds()
			if bounds.Dx() == 0 || bounds.Dy() == 0 {
				fmt.Printf("  Группа %d: пропускаем %s (нулевой размер)\n", groupNum, fileName)
				continue
			}
			images = append(images, img)
			if bounds.Dx() > maxWidth {
				maxWidth = bounds.Dx()
			}
			if bounds.Dy() > maxHeight {
				maxHeight = bounds.Dy()
			}
		}
		if len(images) == 0 {
			fmt.Printf("Группа %d: нет валидных кадров, GIF не создан\n", groupNum)
			continue
		}
		globalPalette, transparentIndex := buildGlobalPalette(images)
		anim := &gif.GIF{
			LoopCount: cfg.LoopCount,
		}
		for _, img := range images {
			canvas := centerImageInCanvas(img, maxWidth, maxHeight, globalPalette, transparentIndex)
			anim.Image = append(anim.Image, canvas)
			anim.Delay = append(anim.Delay, cfg.Delay)
			// Устанавливаем метод удаления: 2 = background (очищать фоном)
			anim.Disposal = append(anim.Disposal, gif.DisposalBackground)
		}
		outName := filepath.Join(cfg.OutputDir, fmt.Sprintf("animation_%02d.gif", groupNum))
		outFile, err := os.Create(outName)
		if err != nil {
			fmt.Printf("Группа %d: ошибка создания файла %s: %v\n", groupNum, outName, err)
			continue
		}
		err = gif.EncodeAll(outFile, anim)
		outFile.Close()
		if err != nil {
			fmt.Printf("Группа %d: ошибка кодирования GIF: %v\n", groupNum, err)
			continue
		}
		fmt.Printf("Группа %d: успешно создан GIF с %d кадрами (размер %dx%d)\n", groupNum, len(images), maxWidth, maxHeight)
	}
	return nil
}

func extractFrameNum(filename string) int {
	parts := strings.Split(strings.TrimSuffix(filename, ".png"), "_")
	if len(parts) != 3 {
		return 0
	}
	num, _ := strconv.Atoi(parts[2])
	return num
}
