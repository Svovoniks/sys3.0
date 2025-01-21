package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"runtime"
	"sort"
	"strings"
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
		exe.execute()
		return true
	}

	cfg.useArgs = false

	if exe, err := getExecutable(launcherFile, cfg); err == nil {
		exe.execute()
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
	env        *Env
	useArgs    bool
	stickyMode bool
	argsUsed   int
}

type Executable struct {
	command   []string
	curThread bool
}

func (e *Executable) execute() {
	if len(e.command) == 0 {
		return
	}

	cmd := exec.Command(e.command[0], e.command[1:]...)
	cmd.Stdout = os.Stdout

	var err error
	if e.curThread {
		cmd.Stdin = os.Stdin
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}

	if err != nil {
		fmt.Printf("Couldn't run the command \"%v\" error: \"%v\"\n", e.command, err)
	}
}

func getWidth() int {
	if ws, err := tsize.GetSize(); err == nil {
		return ws.Width
	} else {
		fmt.Println("Couldn't get terminal size")
		return 200
	}
}

func eraseSelectionScreen(state *SelectionScreenState) {
	width := getWidth()
	for range state.lineConunt - 1 {
		fmt.Print("\r")
		fmt.Print(strings.Repeat(" ", width))
		fmt.Print("\033[F")
	}
}
func showSelectionScreen(state *SelectionScreenState) {
	width := getWidth()

	eraseSelectionScreen(state)

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
			-width,
			fmt.Sprintf("%[1]*s", (width+len(state.options[idx].fileName))/2, state.options[idx].fileName),
		))

		if i == optionCount-1 {
			buffer.WriteString("\033[0m")
		}
	}

	buffer.WriteString("Enter launcher name > ")
	buffer.WriteString(string(state.curText))
	str := buffer.String()
	fmt.Printf(str)
	state.lineConunt = int(math.Ceil(float64(len(str)) / float64(width)))
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

func updateOptions(state *SelectionScreenState) {
	txt := string(state.curText)
	for i := range state.options {
		st := state.options[i].fileName
		st += strings.Repeat(" ", state.maxLen-len(st))
		state.options[i].dist = levenshtein.ComputeDistance(txt, st)
	}
	sort.Sort(state.options)
}

func processRune(rn rune, state *SelectionScreenState) {
	switch {
	case rn == 127: // backpace
		if len(state.curText) > 0 {
			state.curText = (state.curText)[:len(state.curText)-1]
			updateOptions(state)
		}
	case rn == 13: // enter
		state.done = true
		state.result = state.options[0].fileName
	case rn == 3: // ctrl + c
		state.exit = true
	case unicode.IsGraphic(rn):
		state.curText = append(state.curText, rn)
		updateOptions(state)
	}
}

type SelectionScreenState struct {
	options    Options
	curText    []rune
	result     string
	maxLen     int
	lineConunt int
	done       bool
	exit       bool
}

type UserResult struct {
	exit   bool
	result string
}

func getLauncherFromUser(cfg *Config) UserResult {
	state := SelectionScreenState{}
	for _, v := range getAllLauncherNames(cfg) {
		state.options = append(state.options, Option{
			fileName: v,
		})
		if len(v) > state.maxLen {
			state.maxLen = len(v)
		}

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

	for {
		showSelectionScreen(&state)
		rn, _, err := reader.ReadRune()

		if err != nil {
			panic("Couldn't read input")
		}

		processRune(rn, &state)

		if state.done || state.exit {
			state.lineConunt += 1
			eraseSelectionScreen(&state)
			return UserResult{
				exit:   state.exit,
				result: path.Join(cfg.env.launcherDir, state.result+".txt"),
			}
		}
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

	flag.BoolVar(&cfg.stickyMode, "s", true, "if used sys will keep asking for new commands basically like a shell")

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
			exe.execute()
		}

		if !cfg.stickyMode {
			break
		}
	}
}
