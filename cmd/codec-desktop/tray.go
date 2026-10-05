//go:build windows

package main

import (
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/mahasenabheetha/codec/v2/internal/web"
)

// trayRecent is how many recent folders the tray menu lists.
const trayRecent = 5

// hiddenFlag starts codec in the tray without showing the window; the
// login entry uses it.
const hiddenFlag = "--hidden"

// setupTray adds the tray icon: a click shows the window, the menu
// (rebuilt on each open, so recent folders are current) opens a recent
// folder or quits.
func (d *desktop) setupTray() {
	tray := d.app.SystemTray.New()
	if icon, err := web.AppFile("favicon.ico"); err == nil {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("codec")
	menu := d.app.NewMenu()
	tray.SetMenu(menu)
	tray.OnClick(d.showWindow)
	tray.OnRightClick(func() {
		d.fillTrayMenu(menu)
		tray.OpenMenu()
	})
}

func (d *desktop) fillTrayMenu(menu *application.Menu) {
	menu.Clear()
	menu.Add("Open codec").OnClick(func(*application.Context) { d.showWindow() })
	if recent := d.cfg.Get().RecentFolders; len(recent) > 0 {
		menu.AddSeparator()
		for _, dir := range recent[:min(len(recent), trayRecent)] {
			menu.Add(filepath.Base(dir)).OnClick(func(*application.Context) {
				if err := d.server.Load().OpenPath(dir, ""); err != nil {
					logf("open %s: %v", dir, err)
				}
				d.showWindow()
			})
		}
	}
	menu.AddSeparator()
	menu.Add("Quit codec").OnClick(func(*application.Context) { d.quit() })
	menu.Update()
}

// keepInTray hides the window instead of closing it. With "Keep running
// in the tray" off, closing quits through app.Quit instead of letting
// the last window's close end the process: only Quit removes the tray
// icon and runs the shutdown tasks (saving the window).
func (d *desktop) keepInTray() {
	d.win.RegisterHook(eventClosing, func(e *application.WindowEvent) {
		if d.quitting.Load() {
			return // already quitting: let Wails close the window
		}
		e.Cancel()
		if d.cfg.Get().Desktop.QuitOnClose {
			go d.quit()
			return
		}
		d.saver.save()
		d.win.Hide()
	})
}

func (d *desktop) showWindow() {
	application.InvokeAsync(func() {
		if d.win == nil {
			return // a second launch before the window exists
		}
		if d.win.IsMinimised() {
			d.win.UnMinimise()
		}
		d.win.Show()
		d.win.Focus()
	})
}

func (d *desktop) quit() {
	d.quitting.Store(true)
	d.app.Quit()
}
