package gifmaker

import (
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config содержит параметры создания GIF
type Config struct {
	InputDir  string // папка с PNG-файлами вида sprite_XX_YY.png
	OutputDir string // папка для сохранения GIF
	Delay     int    // задержка между кадрами в сотых долях секунды (100 = 1 сек)
	LoopCount int    // 0 = бесконечно, -1 = один раз, иначе LoopCount+1 раз
}

// ProcessAllGroups обрабатывает все группы спрайтов и создаёт для каждой GIF
func ProcessAllGroups(cfg Config) error {
	// Читаем все файлы в InputDir
	entries, err := os.ReadDir(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать папку %s: %w", cfg.InputDir, err)
	}

	// Собираем все PNG-файлы, соответствующие шаблону sprite_XX_YY.png
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

	// Группируем файлы по номеру анимации (первая часть после sprite_)
	groups := make(map[int][]string) // key = номер группы, value = имена файлов
	for _, f := range files {
		// Парсим sprite_02_03.png
		parts := strings.Split(strings.TrimSuffix(f, ".png"), "_")
		if len(parts) != 3 {
			continue // пропускаем файлы с неправильным форматом
		}
		groupNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		groups[groupNum] = append(groups[groupNum], f)
	}

	// Создаём выходную папку
	if err := os.MkdirAll(cfg.OutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("не удалось создать выходную папку: %w", err)
	}

	// Для каждой группы создаём GIF
	for groupNum, fileNames := range groups {
		// Сортируем файлы по номеру кадра (вторая часть)
		sort.Slice(fileNames, func(i, j int) bool {
			frameI := extractFrameNum(fileNames[i])
			frameJ := extractFrameNum(fileNames[j])
			return frameI < frameJ
		})

		// Создаём анимацию
		anim := &gif.GIF{
			LoopCount: cfg.LoopCount,
		}

		for _, fileName := range fileNames {
			path := filepath.Join(cfg.InputDir, fileName)
			// Открываем PNG
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("ошибка открытия %s: %w", path, err)
			}
			img, err := png.Decode(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("ошибка декодирования %s: %w", path, err)
			}

			// Конвертируем в Paletted (GIF требует этого формата)
			palettedImg := image.NewPaletted(img.Bounds(), nil)
			// Простая квантизация — копируем цвета (палитра будет построена автоматически)
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					palettedImg.Set(x, y, img.At(x, y))
				}
			}

			anim.Image = append(anim.Image, palettedImg)
			anim.Delay = append(anim.Delay, cfg.Delay)
		}

		if len(anim.Image) == 0 {
			continue
		}

		// Сохраняем GIF
		outName := filepath.Join(cfg.OutputDir, fmt.Sprintf("animation_%02d.gif", groupNum))
		outFile, err := os.Create(outName)
		if err != nil {
			return fmt.Errorf("ошибка создания %s: %w", outName, err)
		}
		err = gif.EncodeAll(outFile, anim)
		outFile.Close()
		if err != nil {
			return fmt.Errorf("ошибка кодирования GIF для группы %d: %w", groupNum, err)
		}
	}

	return nil
}

// extractFrameNum извлекает номер кадра из имени файла sprite_XX_YY.png
func extractFrameNum(filename string) int {
	parts := strings.Split(strings.TrimSuffix(filename, ".png"), "_")
	if len(parts) != 3 {
		return 0
	}
	num, _ := strconv.Atoi(parts[2])
	return num
}
