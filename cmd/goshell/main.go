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
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "GoShell - 极速轻量远程终端与文件管理",
			Width:   1120,
			Height:  720,
			Content: ui.View(appView.View),
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
