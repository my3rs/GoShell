package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"goshell/assets/brand"
	"goshell/assets/fonts"
	"goshell/assets/natives"
	"goshell/internal/app"
	uiPkg "goshell/internal/ui"
)

func main() {
	if err := natives.Ensure(); err != nil {
		log.Printf("warning: failed to prepare native terminal engine: %v", err)
	}

	if err := fonts.Register(); err != nil {
		log.Printf("warning: failed to register embedded fonts: %v", err)
	}

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
		appView.AttachWindow(win)

		if runtime.GOOS != "darwin" {
			if err := win.SetIcon(brand.IconPNG); err != nil {
				log.Printf("failed to set GoShell window icon: %v", err)
			}
		}

		win.OnClose(func(e *mygo.CloseEvent) {
			if !appView.ConfirmClose() {
				e.PreventDefault()
				return
			}
		})

		win.OnClosed(func() {
			appView.CloseAll()
			mygo.App.Quit()
		})
	})

	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if !appView.ConfirmQuit() {
			e.PreventDefault()
			return
		}
		appView.CloseAll()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
