package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	uiPkg "goshell/internal/ui"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to initialize GoShell: %v", err)
	}
	defer application.Close()

	appView := uiPkg.NewAppView(application)

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:   "GoShell",
			Width:   1120,
			Height:  720,
			Content: ui.View(appView.View),
		})
		win.OnClosed(func() {
			appView.CloseAll()
		})
	})

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		appView.CloseAll()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
