package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gopxl/beep"
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
		// TODO: support .wav and .mp3
		if fileExtension != ".ogg" {
			continue
		}
		songList = append(songList, filepath.Join(dirPath, fileName))
	}
	return songList, nil
}

func createStreamerFromFile(filePath string, playerState PlayerState) (beep.StreamSeekCloser, *beep.Ctrl, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	streamer, format, err := vorbis.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	ctrl := &beep.Ctrl{Streamer: beep.Loop(1, streamer), Paused: !playerState.playing}
	speaker.Play(ctrl)
	return streamer, ctrl, nil
}

func main() {
	playerState := PlayerState{
		playing:          false,
		currentSongIndex: 0,
	}
	songList, err := readSongListFromDir("songs")
	if err != nil {
		log.Fatal(err)
	}
	streamer, ctrl, err := createStreamerFromFile(songList[0], playerState)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()
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
	updateSongList := func(playingIndex int) {
		songListText = ""
		for i, songPath := range songList {
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
	updateSongList(0)
	baseFlex := tview.NewFlex().
		SetDirection(tview.FlexColumnCSS).
		AddItem(songTextView, 0, 1, false)
	baseFlex.SetBorder(true).SetTitle("Music Player")
	playButton := newButton(PlayIcon)
	playButton.SetSelectedFunc(func() {
		speaker.Lock()
		ctrl.Paused = !ctrl.Paused
		speaker.Unlock()
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
		playerState.currentSongIndex = (playerState.currentSongIndex + 1) % len(songList)
		updateSongList(playerState.currentSongIndex)
		streamer.Close()
		streamer, ctrl, err = createStreamerFromFile(songList[playerState.currentSongIndex], playerState)
		if err != nil {
			log.Fatal(err)
		}
	})
	prevButton := newButton(PrevIcon)
	prevButton.SetSelectedFunc(func() {
		playerState.currentSongIndex = (playerState.currentSongIndex - 1 + len(songList)) % len(songList)
		updateSongList(playerState.currentSongIndex)
		streamer.Close()
		streamer, ctrl, err = createStreamerFromFile(songList[playerState.currentSongIndex], playerState)
		if err != nil {
			log.Fatal(err)
		}
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
		return event
	}
	buttonsFlex.SetInputCapture(playerState.OnInput)
	baseFlex.AddItem(buttonsFlex, 0, 1, true)
	app.SetRoot(baseFlex, true).Run()
}
