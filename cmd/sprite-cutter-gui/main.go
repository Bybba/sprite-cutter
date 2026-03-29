package main

import (
	"fmt"
	"sprite-cutter/internal/cutter" // подставь своё название модуля!
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func main() {
	// Создаём приложение
	myApp := app.New()
	myWindow := myApp.NewWindow("Нарезчик спрайтов")
	myWindow.Resize(fyne.NewSize(600, 250))

	// Поле для входного файла + кнопка обзора
	inputEntry := widget.NewEntry()
	inputEntry.SetPlaceHolder("Выберите файл со спрайтами...")
	openFileBtn := widget.NewButton("Обзор...", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if reader == nil {
				return // пользователь отменил выбор
			}
			inputEntry.SetText(reader.URI().Path())
			reader.Close()
		}, myWindow)
	})

	// Поле для выходной папки + кнопка обзора
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Папка для сохранения спрайтов...")
	outputEntry.SetText("images/output") // можно задать значение по умолчанию
	openFolderBtn := widget.NewButton("Обзор...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if uri == nil {
				return
			}
			outputEntry.SetText(uri.Path())
		}, myWindow)
	})

	vertEntry := widget.NewEntry()
	vertEntry.SetText("20")
	vertEntry.SetPlaceHolder("Вертикальный порог (пиксели)")

	horizEntry := widget.NewEntry()
	horizEntry.SetText("50")
	horizEntry.SetPlaceHolder("Горизонтальный порог (пиксели)")

	// Метка для статуса
	statusLabel := widget.NewLabel("Готов к работе")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Кнопка запуска
	var processBtn *widget.Button
	processBtn = widget.NewButton("Нарезать спрайты", func() {
		// Проверяем, что поля заполнены
		if inputEntry.Text == "" {
			dialog.ShowInformation("Ошибка", "Выберите входной файл", myWindow)
			return
		}
		if outputEntry.Text == "" {
			dialog.ShowInformation("Ошибка", "Выберите папку для сохранения", myWindow)
			return
		}

		// Деактивируем кнопку, чтобы не нажали дважды
		processBtn.Disable()
		statusLabel.SetText("Обработка...")

		// Запускаем обработку в горутине, чтобы не блокировать UI
		go func() {
			vert, err := strconv.Atoi(vertEntry.Text)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("неверный вертикальный порог"), myWindow)
					processBtn.Enable()
					statusLabel.SetText("Ошибка: неверный порог")
				})
				return
			}
			horiz, err := strconv.Atoi(horizEntry.Text)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("неверный горизонтальный порог"), myWindow)
					processBtn.Enable()
					statusLabel.SetText("Ошибка: неверный порог")
				})
				return
			}
			err = cutter.Process(
				inputEntry.Text,
				outputEntry.Text,
				0, 0,
				cutter.DefaultThreshold,
				vert, horiz,
			)

			fyne.Do(func() {
				processBtn.Enable()
				if err != nil {
					statusLabel.SetText("Ошибка: " + err.Error())
					dialog.ShowError(err, myWindow)
				} else {
					statusLabel.SetText("Готово! Спрайты сохранены в " + outputEntry.Text)
				}
			})
		}()
	})

	// Собираем layout
	content := container.NewVBox(
		widget.NewLabelWithStyle("Входной файл", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, openFileBtn, inputEntry),
		widget.NewLabelWithStyle("Папка для сохранения", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, openFolderBtn, outputEntry),
		widget.NewLabelWithStyle("Пороги группировки", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Вертикальный:"),
			vertEntry,
			widget.NewLabel("Горизонтальный:"),
			horizEntry,
		),
		processBtn,
		statusLabel,
	)

	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}
