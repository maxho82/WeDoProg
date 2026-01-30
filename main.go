package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	log.Println("=== Запуск WeDoProg - Программирование WeDo 2.0 ===")

	// Создаем приложение
	myApp := app.New()
	myApp.Settings().SetTheme(&CustomTheme{})

	// Создаем главное окно
	window := myApp.NewWindow("WeDoProg - Визуальный программист WeDo 2.0")
	window.SetMaster()
	window.Resize(fyne.NewSize(1400, 900))

	// Инициализируем менеджер хаба
	hubMgr, err := NewHubManager()
	if err != nil {
		log.Fatalf("Ошибка инициализации хаба: %v", err)
	}

	// Создаем GUI
	gui := NewMainGUI(window, hubMgr)

	// Запускаем приложение
	window.SetContent(gui.BuildUI())
	window.ShowAndRun()

	// Отключаемся при выходе
	hubMgr.Disconnect()
}
