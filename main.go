package main

import (
	"gomp3/filepicker"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/rivo/tview"
)

const (
	PlayIcon  string = "⏵"
	PauseIcon string = "⏸"
	StopIcon  string = "⏹"
	NextIcon  string = "⏭"
	PrevIcon  string = "⏮"
)

type PlayerControls struct {
	cursor  int
	buttons []tview.Primitive
}

type PlayerState struct {
	playing          bool
	currentSongIndex int
	controls         PlayerControls
	songList         []string
	streamer         beep.StreamSeekCloser
	ctrl             *beep.Ctrl
	OnInput          func(event *tcell.EventKey) *tcell.EventKey
}

func readSongListFromDir(dirPath string) ([]string, error) {
	dir, err := os.ReadDir(dirPath)
	if err != nil {
		return []string{}, err
	}
	songList := []string{}
	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		fileExtension := filepath.Ext(fileName)
		if fileExtension != ".ogg" && fileExtension != ".mp3" {
			continue
		}
		songList = append(songList, filepath.Join(dirPath, fileName))
	}
	return songList, nil
}

func (ps *PlayerState) PlaySong() bool {
	if ps.ctrl == nil {
		return false
	}
	speaker.Lock()
	ps.ctrl.Paused = !ps.ctrl.Paused
	speaker.Unlock()
	return true
}

func (ps *PlayerState) NextSong() bool {
	if len(ps.songList) == 0 {
		return false
	}
	ps.currentSongIndex = (ps.currentSongIndex + 1) % len(ps.songList)
	ps.createStreamerFromFile()
	return true
}

func (ps *PlayerState) PrevSong() bool {
	if len(ps.songList) == 0 {
		return false
	}
	ps.currentSongIndex = (ps.currentSongIndex - 1 + len(ps.songList)) % len(ps.songList)
	ps.createStreamerFromFile()
	return true
}

func (ps *PlayerState) UpdateSongList(songList []string) {
	ps.songList = songList
	ps.currentSongIndex = 0
	ps.createStreamerFromFile()
}

func (ps *PlayerState) createStreamerFromFile() error {
	if ps.streamer != nil {
		ps.streamer.Close()
	}
	filePath := ps.songList[ps.currentSongIndex]
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	extension := filepath.Ext(filePath)
	var streamer beep.StreamSeekCloser
	var format beep.Format
	if extension == ".mp3" {
		streamer, format, err = mp3.Decode(f)
	} else if extension == ".ogg" {
		streamer, format, err = vorbis.Decode(f)
	}
	if err != nil {
		log.Fatal(err)
	}
	ps.streamer = streamer
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, streamer), Paused: !ps.playing}
	ps.ctrl = ctrl
	speaker.Play(ctrl)
	return nil
}

func (ps *PlayerState) Shutdown() {
	if ps.streamer != nil {
		ps.streamer.Close()
	}
}

func main() {
	playerState := PlayerState{
		playing:          false,
		currentSongIndex: 0,
		songList:         []string{},
	}
	defer playerState.Shutdown()
	newButton := func(label string) *tview.Button {
		button := tview.NewButton(label)
		button.SetBackgroundColor(tcell.ColorBlue)
		button.SetBackgroundColorActivated(tcell.ColorDarkBlue)
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
		for i, songPath := range playerState.songList {
			songName := filepath.Base(songPath)
			if i == playingIndex {
				songListText += "[\"active\"][yellow]" + songName + "[\"\"]\n"
			} else {
				songListText += "[white]" + songName + "\n"
			}
		}
		songTextView.SetText(songListText)
		songTextView.Highlight("active")
		songTextView.ScrollToHighlight()
	}

	baseFlex := tview.NewFlex().
		SetDirection(tview.FlexColumnCSS).
		AddItem(songTextView, 0, 1, false)
	baseFlex.SetBorder(true).SetTitle("Music Player")
	playButton := newButton(PlayIcon)
	playButton.SetSelectedFunc(func() {
		success := playerState.PlaySong()
		if !success {
			return
		}
		if playerState.playing {
			playerState.playing = false
			playButton.SetLabel(PlayIcon)
			return
		} else {
			playerState.playing = true
			playButton.SetLabel(PauseIcon)
			return
		}
	})
	nextButton := newButton(NextIcon)
	nextButton.SetSelectedFunc(func() {
		success := playerState.NextSong()
		if !success {
			return
		}
		refreshSongList(playerState.currentSongIndex)
	})
	prevButton := newButton(PrevIcon)
	prevButton.SetSelectedFunc(func() {
		success := playerState.PrevSong()
		if !success {
			return
		}
		refreshSongList(playerState.currentSongIndex)
	})
	controls := PlayerControls{
		cursor: 1,
		buttons: []tview.Primitive{
			prevButton,
			playButton,
			nextButton,
		},
	}
	playerState.controls = controls
	buttonsFlex := tview.NewFlex()
	buttonsFlex.SetBorder(true)
	for i, btn := range controls.buttons {
		buttonsFlex.AddItem(btn, 0, 1, i == controls.cursor)
	}
	playerState.OnInput = func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			controls.cursor = (controls.cursor + 1) % len(controls.buttons)
			app.SetFocus(buttonsFlex.GetItem(controls.cursor))
			return nil
		} else if event.Key() == tcell.KeyLeft {
			controls.cursor = (controls.cursor - 1 + len(controls.buttons)) % len(controls.buttons)
			app.SetFocus(buttonsFlex.GetItem(controls.cursor))
			return nil
		}

		if event.Key() == tcell.KeyEscape {
			filepicker.Open(app, baseFlex, func(path string) {
				songList, err := readSongListFromDir(path)
				if err != nil {
					log.Fatal(err)
				}
				if len(songList) == 0 {
					return
				}
				playerState.UpdateSongList(songList)
				refreshSongList(0)
			})
		}
		return event
	}
	buttonsFlex.SetInputCapture(playerState.OnInput)
	baseFlex.AddItem(buttonsFlex, 0, 1, true)
	err := app.SetRoot(baseFlex, true).Run()
	if err != nil {
		log.Fatal(err)
	}
}
