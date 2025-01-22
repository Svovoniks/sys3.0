package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode"

	"github.com/agnivade/levenshtein"
	tsize "github.com/kopoli/go-terminal-size"
	"golang.org/x/term"
)

const maxOnScreenOptionCount = 20

func solveEscape(exe *Executable, escapeSq string, cfg *Config) (string, error) {
	ls := strings.Split(escapeSq, ":")

	switch ls[0] {
	case "input":
		{
			if cfg.useArgs {
				if len(flag.Args()) > cfg.argsUsed {
					cfg.argsUsed += 1
					return flag.Arg(cfg.argsUsed - 1), nil
				} else {
					return "", errors.New("Not enough args")
				}
			} else {
				fmt.Printf("Enter %v > ", ls[1])

				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					return scanner.Text(), nil
				}
				panic("Couldn't read input")
			}
		}
	case "util":
		{
			return path.Join(cfg.env.utilDir, ls[1]), nil
		}

	case "launcher":
		{
			return path.Join(cfg.env.launcherDir, ls[1]), nil
		}
	case "this":
		{
			return os.Getwd()
		}
	case "sys_base":
		{
			return cfg.env.baseDir, nil
		}
	case "cur_thr":
		{
			exe.curThread = true
			return "", nil
		}
	}

	return "", fmt.Errorf("Unknown escape key %v", errors.ErrUnsupported)

}

func getExecutable(fileName string, cfg *Config) (*Executable, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic("Couldn't read launcher file")
	}

	runes := []rune(string(data))

	escapeStartIdx := 0
	readingEscape := false
	justAppend := false

	exe := &Executable{}
	buff := bytes.Buffer{}

	for i := range runes {
		if justAppend {
			buff.WriteRune(runes[i])
			justAppend = false
			continue
		}

		if runes[i] == '\\' {
			justAppend = true
			continue
		}

		if runes[i] == '{' {
			readingEscape = true
			escapeStartIdx = i
			continue
		}

		if runes[i] == '}' {
			readingEscape = false
			res, err := solveEscape(exe, string(runes[escapeStartIdx+1:i]), cfg)
			if err != nil {
				return nil, errors.New("Couldn't solve ecape sequence")
			}
			buff.WriteString(res)
			continue
		}

		if readingEscape {
			continue
		}

		if runes[i] == ' ' {
			if buff.Len() != 0 {
				exe.command = append(exe.command, buff.String())
				buff.Reset()
			}
			continue
		}

		buff.WriteRune(runes[i])
	}

	if buff.Len() != 0 {
		exe.command = append(exe.command, buff.String())
	}

	if len(exe.command) > 0 {
		st := exe.command[len(exe.command)-1]
		if len(st) > 0 && st[len(st)-1] == '\n' {
			exe.command[len(exe.command)-1] = st[:len(st)-1]
		}
	}

	return exe, nil

}

func getAllLauncherNames(cfg *Config) []string {
	files, _ := os.ReadDir(cfg.env.launcherDir)

	realFiles := []string{}

	for _, file := range files {
		if spl := strings.Split(file.Name(), "."); !file.IsDir() && len(spl) > 1 && spl[len(spl)-1] == "txt" {
			realFiles = append(realFiles, strings.Join(spl[:len(spl)-1], ""))
		}
	}
	return realFiles
}

func getLauncherFile(name string, cfg *Config) (string, error) {
	for _, file := range getAllLauncherNames(cfg) {
		if file == name {
			return path.Join(cfg.env.launcherDir, name+".txt"), nil
		}
	}

	return "", errors.New("No such launcher")
}

func tryExecuteArgs(cfg *Config) bool {
	if len(flag.Args()) == 0 {
		return false
	}

	launcherFile, err := getLauncherFile(flag.Arg(0), cfg)
	if err != nil {
		return false
	}

	if exe, err := getExecutable(launcherFile, cfg); err == nil {
		exe.execute(cfg.debugMode)
		return true
	}

	cfg.useArgs = false

	if exe, err := getExecutable(launcherFile, cfg); err == nil {
		exe.execute(cfg.debugMode)
		return true
	}

	return false
}

type Env struct {
	baseDir     string
	launcherDir string
	utilDir     string
}

func getEnv() (*Env, error) {
	baseDir := os.Getenv("sys")
	if baseDir == "" {
		return nil, errors.New("Not installed")
	}

	return &Env{
		baseDir:     baseDir,
		launcherDir: path.Join(baseDir, "launchers"),
		utilDir:     path.Join(baseDir, "utils"),
	}, nil
}

type Config struct {
	env            *Env
	useArgs        bool
	stickyMode     bool
	fullScreenMode bool
	debugMode      bool
	argsUsed       int
}

type Executable struct {
	command   []string
	curThread bool
}

func (e *Executable) execute(debugMode bool) {
	if len(e.command) == 0 {
		return
	}

	if debugMode {
		fmt.Printf("About to run: %v\n", strings.Join(e.command, "|"))
	}

	cmd := exec.Command(e.command[0], e.command[1:]...)

	var err error
	if e.curThread {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

		errChan := make(chan error, 1)
		go func() {
			errChan <- cmd.Run()
		}()

		select {
		case sig := <-signalChan:
			_ = cmd.Process.Signal(sig)
		case err = <-errChan:
		}

		signal.Stop(signalChan)
		close(signalChan)
	} else {
		err = cmd.Start()
	}

	if err != nil {
		fmt.Printf("Couldn't run the command \"%v\" error: \"%v\"\n", e.command, err)
	}
}

func eraseFullScreen(state *SelectionScreenState) {
	state.RLock()
	defer state.RUnlock()

	for range state.height {
		fmt.Print(strings.Repeat(" ", state.width))
	}

	for range 2 * state.height {
		fmt.Print("\r")
		fmt.Print(strings.Repeat(" ", state.width))
		fmt.Print("\033[F")
	}
}

func eraseSelectionScreen(state *SelectionScreenState) {
	state.RLock()
	defer state.RUnlock()

	toBeErased := state.lineCount - 1
	if state.fullScreenMode {
		toBeErased = state.height
	}

	for range toBeErased {
		fmt.Print("\r")
		fmt.Print(strings.Repeat(" ", state.width))
		fmt.Print("\033[F")
	}
}

func showSelectionScreen(state *SelectionScreenState) {
	if state.lineCount > 0 {
		eraseSelectionScreen(state)
	}

	state.Lock()
	defer state.Unlock()

	buffer := bytes.Buffer{}
	buffer.WriteRune('\r')
	optionCount := min(maxOnScreenOptionCount, len(state.options))

	for i := range optionCount {
		idx := optionCount - 1 - i

		if i == optionCount-1 {
			buffer.WriteString("\033[48;5;15m\033[38;5;0m")
		}

		buffer.WriteString(fmt.Sprintf(
			"%[1]*s",
			-state.width,
			fmt.Sprintf("%[1]*s", (state.width+len(state.options[idx].fileName))/2, state.options[idx].fileName),
		))

		if i == optionCount-1 {
			buffer.WriteString("\033[0m")
		}
	}

	buffer.WriteString("Enter launcher name > ")
	buffer.WriteString(string(state.curText))
	str := buffer.String()
	fmt.Print(str)
	state.lineCount = int(math.Ceil(float64(len(str)) / float64(state.width)))
}

type Option struct {
	fileName string
	dist     int
}

type Options []Option

func (o Options) Len() int {
	return len(o)
}

func (o Options) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o Options) Less(i, j int) bool {
	return o[i].dist < o[j].dist
}

func updateOptions(options Options, txt string, mxLen int) {

	for i := range options {
		st := options[i].fileName
		st += strings.Repeat(" ", mxLen-len(st))
		options[i].dist = levenshtein.ComputeDistance(txt, st)
	}
	sort.Sort(options)
}

func processRune(rn rune, state *SelectionScreenState) {
	state.Lock()
	defer state.Unlock()

	switch {
	case rn == 127: // backpace
		if len(state.curText) > 0 {
			state.curText = (state.curText)[:len(state.curText)-1]
			updateOptions(state.options, string(state.curText), state.maxLen)
		}
	case rn == 13: // enter
		state.done = true
		state.result = state.options[0].fileName
	case rn == 3: // ctrl + c
		state.exit = true
	case unicode.IsGraphic(rn):
		state.curText = append(state.curText, rn)
		updateOptions(state.options, string(state.curText), state.maxLen)
	}
}

type SelectionScreenState struct {
	sync.RWMutex
	options        Options
	curText        []rune
	result         string
	maxLen         int
	lineCount      int
	width          int
	height         int
	fullScreenMode bool
	done           bool
	exit           bool
}

type UserResult struct {
	exit   bool
	result string
}

func getLauncherFromUser(cfg *Config) UserResult {
	state := SelectionScreenState{fullScreenMode: cfg.fullScreenMode}
	for _, v := range getAllLauncherNames(cfg) {
		state.options = append(state.options, Option{
			fileName: v,
		})
		if len(v) > state.maxLen {
			state.maxLen = len(v)
		}

	}

	if ws, err := tsize.GetSize(); err == nil {
		state.width = ws.Width
		state.height = ws.Height
	} else {
		fmt.Println("Couldn't get terminal size")
		state.width = 200
		state.height = 200
	}

	if len(state.options) == 0 {
		fmt.Println("You don't have any launchers, try adding some and come back")
		return UserResult{exit: true}
	}

	if runtime.GOOS == "linux" {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Println("Couldn't switch to raw terminal mode, suggestions are unavailable")
		} else {
			defer func() {
				err := term.Restore(int(os.Stdin.Fd()), oldState)
				if err != nil {
					fmt.Printf("Couldn't switch back to normal terminal mode\nYou should restart your terminal session\nHere's an error message: %v\n", err)
				}
			}()
		}

	}

	fmt.Print("\n")

	reader := bufio.NewReader(os.Stdin)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if state.fullScreenMode {
		eraseFullScreen(&state)
	}

	go func() {
		if ch, err := tsize.NewSizeListener(); err == nil {
			for {
				select {
				case val, ok := <-ch.Change:
					if !ok {
						return
					}
					state.Lock()
					w := val.Width
					state.height = val.Height
					state.lineCount = max(int(math.Ceil(float64(state.width*state.lineCount)/float64(w))), state.lineCount) + 1
					state.width = w
					state.Unlock()

					eraseSelectionScreen(&state)

					state.Lock()
					state.lineCount = 0
					state.Unlock()

					showSelectionScreen(&state)
				case <-ctx.Done():
					return
				}

			}
		}
	}()

	for {
		showSelectionScreen(&state)
		rn, _, err := reader.ReadRune()

		if err != nil {
			panic("Couldn't read input")
		}

		processRune(rn, &state)

		print("hey")
		state.Lock()
		if state.done || state.exit {
			state.lineCount += 1
			state.Unlock()

			eraseSelectionScreen(&state)

			return UserResult{
				exit:   state.exit,
				result: path.Join(cfg.env.launcherDir, state.result+".txt"),
			}
		}
		state.Unlock()
	}
}

func getConfig() *Config {
	env, err := getEnv()

	if err != nil {
		log.Fatal("sys is not installed")
	}

	cfg := &Config{
		env:     env,
		useArgs: true,
	}

	flag.BoolVar(&cfg.stickyMode, "s", false, "if used sys will keep asking for new commands basically like a shell")
	flag.BoolVar(&cfg.fullScreenMode, "f", false, "if used sys will erase everything from the screen when it asks you for a launcher name")
	flag.BoolVar(&cfg.debugMode, "d", false, "enable debug mode")
	flag.Parse()

	return cfg
}

func main() {
	cfg := getConfig()

	if tryExecuteArgs(cfg) && !cfg.stickyMode {
		return
	}

	cfg.useArgs = false

	for {
		res := getLauncherFromUser(cfg)
		if res.exit {
			break
		}

		if exe, err := getExecutable(res.result, cfg); err == nil {
			exe.execute(cfg.debugMode)
		}

		if !cfg.stickyMode {
			break
		}
	}
}
