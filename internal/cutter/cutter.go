package cutter

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
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
// Каждый прямоугольник ограничивает связную область не-фона (спрайт).
func FindComponents(img image.Image, bgColor color.RGBA, threshold uint32) ([]image.Rectangle, []int) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Инициализируем все метки как -1 (фон)
	labels := make([]int, width*height)
	for i := range labels {
		labels[i] = -1
	}

	// Направления для 8-связности (сдвиги по x, y)
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

			// Нашли новый компонент — запускаем BFS
			queue := [][2]int{{x, y}}
			labels[idx] = currentLabel
			minX, minY := x, y
			maxX, maxY := x, y

			for len(queue) > 0 {
				// Извлекаем из очереди
				px, py := queue[0][0], queue[0][1]
				queue = queue[1:]

				// Обновляем границы компонента
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

				// Проверяем всех соседей (8 направлений)
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

// GroupSprites группирует спрайты в анимации на основе их позиции на исходном листе.
// Сначала группирует по вертикали (строки), затем внутри строки разделяет по горизонтальному разрыву.
// verticalThreshold   — максимальное вертикальное расстояние между нижней гранью предыдущего и верхней гранью следующего,
//
//	чтобы считать их одной строкой.
//
// horizontalThreshold — максимальное горизонтальное расстояние между правой гранью предыдущего и левой гранью следующего,
//
//	чтобы считать их одной анимацией в пределах строки.
//
// Возвращает срез срезов прямоугольников: каждая внутренняя группа — кадры одной анимации.
func GroupSprites(rects []image.Rectangle, verticalThreshold, horizontalThreshold int) [][]image.Rectangle {
	if len(rects) == 0 {
		return nil
	}
	// Сортируем по Y (верхняя граница), затем по X
	sorted := make([]image.Rectangle, len(rects))
	copy(sorted, rects)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Min.Y != sorted[j].Min.Y {
			return sorted[i].Min.Y < sorted[j].Min.Y
		}
		return sorted[i].Min.X < sorted[j].Min.X
	})

	var groups [][]image.Rectangle
	currentGroup := []image.Rectangle{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		prev := currentGroup[len(currentGroup)-1]
		curr := sorted[i]
		verticalGap := curr.Min.Y - prev.Max.Y
		horizontalGap := curr.Min.X - prev.Max.X
		// Если вертикально близки И горизонтально не слишком далеко -> одна группа
		if verticalGap < verticalThreshold && horizontalGap < horizontalThreshold {
			currentGroup = append(currentGroup, curr)
		} else {
			groups = append(groups, currentGroup)
			currentGroup = []image.Rectangle{curr}
		}
	}
	groups = append(groups, currentGroup)

	// Внутри каждой группы сортируем по X (кадры должны идти слева направо)
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Min.X < group[j].Min.X
		})
	}
	return groups
}

// SaveSpriteWithAlphaByColor сохраняет спрайт, делая прозрачными пиксели фона (по цвету).
// В отличие от SaveSpriteWithAlpha, не требует карты меток и работает напрямую с цветом фона.
func SaveSpriteWithAlphaByColor(img image.Image, rect image.Rectangle, bgColor color.RGBA, threshold uint32, outputPath string) error {
	// Создаём новое изображение размером с rect (сдвигаем rect к (0,0))
	newImg := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))

	// Проходим по всем пикселям внутри rect
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.At(x, y)
			if !isBackground(c, bgColor, threshold) {
				// Пиксель принадлежит спрайту — копируем цвет
				newImg.Set(x-rect.Min.X, y-rect.Min.Y, c)
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

// Process основная функция: загружает картинку, находит компоненты, группирует их в анимации
// и сохраняет каждый спрайт в отдельный PNG с прозрачным фоном.
// Имя файла формируется как sprite_<группа>_<кадр>.png.
func Process(inputPath, outputDir string, bgX, bgY int, threshold uint32) error {
	// Открываем файл
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Декодируем изображение
	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	// Получаем цвет фона по указанной точке
	bgColor := color.RGBAModel.Convert(img.At(bgX, bgY)).(color.RGBA)

	// Получаем все прямоугольники связных компонентов (спрайтов)
	components, _ := FindComponents(img, bgColor, threshold)

	// Группируем их в анимации
	// Параметры: вертикальный порог 20, горизонтальный порог 50 — можно вынести в аргументы
	groups := GroupSprites(components, 20, 50)

	// Создаём выходную папку, если её нет
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}

	// Сохраняем каждый спрайт с именем sprite_<группа>_<кадр>.png
	for groupIdx, group := range groups {
		for frameIdx, rect := range group {
			filename := filepath.Join(outputDir, fmt.Sprintf("sprite_%02d_%02d.png", groupIdx+1, frameIdx+1))
			if err := SaveSpriteWithAlphaByColor(img, rect, bgColor, threshold, filename); err != nil {
				return err
			}
		}
	}
	return nil
}
