package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"goshell/assets/brand"
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
		if runtime.GOOS == "darwin" {
			if err := mygo.App.Dock.SetIcon(brand.IconPNG); err != nil {
				log.Printf("failed to set GoShell Dock icon: %v", err)
			}
		}
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:   "GoShell",
			Width:   1120,
			Height:  720,
			Content: ui.View(appView.View),
		})
		if runtime.GOOS != "darwin" {
			if err := win.SetIcon(brand.IconPNG); err != nil {
				log.Printf("failed to set GoShell window icon: %v", err)
			}
		}
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
