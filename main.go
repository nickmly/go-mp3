package main

import (
	"fmt"
	"gomp3/filepicker"
	"gomp3/player"
	"log"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func createVolumeControl(app *tview.Application) (*tview.Flex, *tview.TextView) {
	volumeBgColor := tcell.ColorDarkSlateGray
	volumeFlex := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	volumeTextView := tview.NewTextView()
	volumeTextView.SetTextAlign(tview.AlignRight)
	volumeTextView.SetBackgroundColor(volumeBgColor)
	volumeTextView.SetText("Volume: 100%\t")
	volumeTextView.SetChangedFunc(func() {
		app.Draw()
	})
	volumeFlex.AddItem(tview.NewBox().SetBackgroundColor(volumeBgColor), 0, 1, false)
	volumeFlex.AddItem(volumeTextView, 0, 1, false)
	return volumeFlex, volumeTextView
}

func createInstructions() *tview.Flex {
	instructions := []string{
		"[esc] select folder",
		"[←/→] select button",
		"[+/-] volume control",
	}
	instructionsBgColor := tcell.ColorDarkGray
	instructionsFlex := tview.NewFlex()
	for _, i := range instructions {
		tv := tview.NewTextView()
		tv.SetTextAlign(tview.AlignCenter)
		tv.SetBackgroundColor(instructionsBgColor)
		tv.SetText(i)
		f := tview.NewFlex()
		f.SetDirection(tview.FlexColumnCSS)
		f.AddItem(tview.NewBox().SetBackgroundColor(instructionsBgColor), 0, 1, false)
		f.AddItem(tv, 0, 1, false)
		f.AddItem(tview.NewBox().SetBackgroundColor(instructionsBgColor), 0, 1, false)
		instructionsFlex.AddItem(f, 0, 1, false)
	}
	return instructionsFlex
}

func main() {
	playerState := player.NewPlayerState()
	defer playerState.Shutdown()
	newButton := func(label string) *tview.Button {
		button := tview.NewButton(label)
		button.SetBackgroundColor(tcell.ColorDarkSlateBlue)
		button.SetBackgroundColorActivated(tcell.ColorLightSteelBlue)
		button.SetLabelColorActivated(tcell.ColorWhite)
		return button
	}
	app := tview.NewApplication()
	songTextView := tview.NewTextView()
	songTextView.SetScrollable(true)
	songTextView.SetDynamicColors(true)
	songTextView.SetRegions(true)
	songTextView.SetChangedFunc(func() {
		app.Draw()
	})
	songListText := ""
	refreshSongList := func(playingIndex int) {
		songListText = ""
		for i, songPath := range playerState.CurrentSongList() {
			songName := filepath.Base(songPath)
			if i == playingIndex {
				songListText += "[\"active\"][yellow]" + songName + "[\"\"]\n"
			} else {
				songListText += "[white]" + songName + "\n"
			}
		}
		songTextView.Clear()
		songTextView.SetText(songListText)
		songTextView.Highlight("active")
		songTextView.ScrollToHighlight()
	}

	baseFlex := tview.NewFlex().
		SetDirection(tview.FlexColumnCSS).
		AddItem(songTextView, 0, 6, false)
	baseFlex.SetBorder(true).SetTitle("Music Player")
	volumeFlex, volumeTextView := createVolumeControl(app)
	baseFlex.AddItem(volumeFlex, 0, 2, false)
	playButton := newButton(player.PlayIcon)
	playButton.SetSelectedFunc(func() {
		success := playerState.PlaySong()
		if !success {
			return
		}
		if playerState.IsPlaying() {
			playButton.SetLabel(player.PlayIcon)
		} else {
			playButton.SetLabel(player.PauseIcon)
		}
		playerState.TogglePlaying()
	})
	nextButton := newButton(player.NextIcon)
	nextButton.SetSelectedFunc(func() {
		success := playerState.NextSong()
		if !success {
			return
		}
		refreshSongList(playerState.CurrentSongIndex())
	})
	prevButton := newButton(player.PrevIcon)
	prevButton.SetSelectedFunc(func() {
		success := playerState.PrevSong()
		if !success {
			return
		}
		refreshSongList(playerState.CurrentSongIndex())
	})
	buttons := []tview.Primitive{
		prevButton,
		playButton,
		nextButton,
	}
	startCursor := 1
	controls := playerState.AddPlayerControls(startCursor, buttons)
	buttonsFlex := tview.NewFlex()
	for i, btn := range buttons {
		buttonsFlex.AddItem(btn, 0, 1, i == startCursor)
	}
	playerState.OnInput = func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			cursor := controls.GoRight()
			app.SetFocus(buttonsFlex.GetItem(cursor))
			return nil
		} else if event.Key() == tcell.KeyLeft {
			cursor := controls.GoLeft()
			app.SetFocus(buttonsFlex.GetItem(cursor))
			return nil
		}

		if event.Key() == tcell.KeyEscape {
			filepicker.Open(app, baseFlex, func(path string) {
				_, err := playerState.ReadSongListFromDir(path)
				if err != nil {
					// TODO: show modal
				} else {
					refreshSongList(0)
				}
			})
		}

		// Plus
		if event.Rune() == 61 {
			v := playerState.IncreaseVolume()
			volumeTextView.SetText(fmt.Sprintf("Volume: %.1f%%", v))
		}
		// Minus
		if event.Rune() == 45 {
			v := playerState.DecreaseVolume()
			volumeTextView.SetText(fmt.Sprintf("Volume: %.1f%%", v))
		}
		return event
	}
	playerState.OnSongChanged = func() {
		refreshSongList(playerState.CurrentSongIndex())
	}
	buttonsFlex.SetInputCapture(playerState.OnInput)
	baseFlex.AddItem(buttonsFlex, 0, 4, true)
	instructionsFlex := createInstructions()
	baseFlex.AddItem(instructionsFlex, 0, 1, false)
	err := app.SetRoot(baseFlex, true).Run()
	if err != nil {
		log.Fatal(err)
	}
}
