package player

import (
	"errors"
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

func (pc *PlayerControls) GoRight() int {
	pc.cursor = (pc.cursor + 1) % len(pc.buttons)
	return pc.cursor
}

func (pc *PlayerControls) GoLeft() int {
	pc.cursor = (pc.cursor - 1 + len(pc.buttons)) % len(pc.buttons)
	return pc.cursor
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

func NewPlayerState() *PlayerState {
	return &PlayerState{
		playing:          false,
		currentSongIndex: 0,
		songList:         []string{},
	}
}

func (ps *PlayerState) AddPlayerControls(startCursor int, buttons []tview.Primitive) PlayerControls {
	pc := PlayerControls{
		cursor:  startCursor,
		buttons: buttons,
	}
	ps.controls = pc
	return pc
}

func (ps *PlayerState) ReadSongListFromDir(dirPath string) ([]string, error) {
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
	if len(songList) == 0 {
		return []string{}, errors.New("song list is empty")
	}
	ps.songList = songList
	ps.updateSongList(songList)
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

func (ps *PlayerState) updateSongList(songList []string) {
	if len(ps.songList) == 0 {
		return
	}
	ps.songList = songList
	ps.currentSongIndex = 0
	ps.createStreamerFromFile()
}

func (ps *PlayerState) Shutdown() {
	if ps.streamer != nil {
		ps.streamer.Close()
	}
}

func (ps *PlayerState) TogglePlaying() {
	ps.playing = !ps.playing
}

func (ps *PlayerState) IsPlaying() bool {
	return ps.playing
}

func (ps *PlayerState) CurrentSongList() []string {
	return ps.songList
}

func (ps *PlayerState) CurrentSongIndex() int {
	return ps.currentSongIndex
}
