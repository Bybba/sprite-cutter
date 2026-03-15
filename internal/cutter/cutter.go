package cutter

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// Параметры по умолчанию
const DefaultThreshold = 50 // допуск по сумме разностей RGB (0-255*3)

// isBackground проверяет, является ли цвет фоновым с учётом порога
func isBackground(c color.Color, bgColor color.RGBA, threshold uint32) bool {
	r, g, b, _ := c.RGBA()
	// сдвигаем 8 бит, чтобы получить 0-255
	dr := absDiff(int(r>>8), int(bgColor.R))
	dg := absDiff(int(g>>8), int(bgColor.G))
	db := absDiff(int(b>>8), int(bgColor.B))
	return uint32(dr+dg+db) < threshold
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// FindComponents возвращает список прямоугольников и метки пикселей (индекс компонента или -1)
func FindComponents(img image.Image, bgColor color.RGBA, threshold uint32) ([]image.Rectangle, []int) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Инициализируем все метки как -1 (фон)
	labels := make([]int, width*height)
	for i := range labels {
		labels[i] = -1
	}

	dirs := [][]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
	var components []image.Rectangle
	currentLabel := 0 // текущий индекс компонента

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if labels[idx] != -1 {
				continue // уже обработали
			}

			// Получаем цвет пикселя
			c := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			if isBackground(c, bgColor, threshold) {
				// Фон: метка уже -1, просто идём дальше
				continue
			}

			// Нашли новый компонент
			queue := [][2]int{{x, y}}
			labels[idx] = currentLabel
			minX, minY := x, y
			maxX, maxY := x, y

			for len(queue) > 0 {
				// Извлекаем из очереди
				px, py := queue[0][0], queue[0][1]
				queue = queue[1:]

				// Обновляем границы
				if px < minX {
					minX = px
				}
				if py < minY {
					minY = py
				}
				if px > maxX {
					maxX = px
				}
				if py > maxY {
					maxY = py
				}

				// Проверяем всех соседей
				for _, d := range dirs {
					nx, ny := px+d[0], py+d[1]
					if nx < 0 || nx >= width || ny < 0 || ny >= height {
						continue
					}
					nIdx := ny*width + nx
					if labels[nIdx] != -1 {
						continue // уже обработан
					}

					nc := img.At(nx+bounds.Min.X, ny+bounds.Min.Y)
					if !isBackground(nc, bgColor, threshold) {
						// Сосед — часть спрайта
						labels[nIdx] = currentLabel
						queue = append(queue, [2]int{nx, ny})
					} else {
						// Сосед — фон, помечаем как -1 (можно не менять, т.к. уже -1)
					}
				}
			}

			// Сохраняем прямоугольник компонента (в координатах исходного изображения)
			rect := image.Rect(
				bounds.Min.X+minX,
				bounds.Min.Y+minY,
				bounds.Min.X+maxX+1,
				bounds.Min.Y+maxY+1,
			)
			components = append(components, rect)
			currentLabel++
		}
	}

	return components, labels
}

// SaveSprite сохраняет область изображения как PNG
func SaveSprite(img image.Image, rect image.Rectangle, outputPath string) error {
	// Создаем под-изображение
	subImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)

	// Создаем файл
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Кодируем в PNG
	return png.Encode(outFile, subImg)
}

// Process основная функция: загружает картинку, находит компоненты, сохраняет спрайты с прозрачностью
func Process(inputPath, outputDir string, bgX, bgY int, threshold uint32) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	bgColor := color.RGBAModel.Convert(img.At(bgX, bgY)).(color.RGBA)

	components, labels := FindComponents(img, bgColor, threshold)

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}

	for i, rect := range components {
		filename := filepath.Join(outputDir, fmt.Sprintf("sprite_%04d.png", i))
		if err := SaveSpriteWithAlpha(img, labels, i, rect, filename); err != nil {
			return err
		}
	}

	return nil
}

// SaveSpriteWithAlpha сохраняет спрайт с прозрачным фоном, используя карту меток
func SaveSpriteWithAlpha(img image.Image, labels []int, compIndex int, rect image.Rectangle, outputPath string) error {
	bounds := img.Bounds()
	width := bounds.Dx() // ширина исходного изображения

	// Создаём новое изображение размером с rect (сдвигаем rect к (0,0))
	newImg := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))

	// Проходим по всем пикселям внутри rect
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// Индекс в массиве labels
			idx := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
			if labels[idx] == compIndex {
				// Пиксель принадлежит спрайту — копируем цвет
				// В новом изображении координаты смещены: (x - rect.Min.X, y - rect.Min.Y)
				newImg.Set(x-rect.Min.X, y-rect.Min.Y, img.At(x, y))
			}
			// Иначе оставляем прозрачным (по умолчанию NRGBA уже имеет A=0)
		}
	}

	// Создаём файл
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Кодируем в PNG
	return png.Encode(outFile, newImg)
}
