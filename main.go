package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"nebula_client/internal/app"
	"nebula_client/internal/model"
)

func main() {
	application := app.NewApp(model.NewDefaultConfigStore())

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:         "NebulaDesk — Native NebulaGraph Client",
			Width:         1380,
			Height:        880,
			MinWidth:      920,
			MinHeight:     620,
			StateKey:      "nebuladesk-main",
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(application.View),
		})
		application.Win = win
		win.OnClose(func(_ *mygo.CloseEvent) {
			if application.Client != nil {
				application.Client.Close()
			}
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
