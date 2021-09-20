package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"math/rand"
	"os"
	"time"
)
import "flag"

type WordGame struct {
	buttons map[string]*widget.Button
	labels  map[string]*widget.Label
	window  fyne.Window

	wordDistribution *WordDistribution
	currentWordId    int
	currentQueryLang string
}

type Word struct {
	A    string  `json:"a"`
	B    string  `json:"b"`
	Freq float64 `json:"freq"`
}

type Action struct {
	Id        int  `json:"id"`
	IsCorrect bool `json:"is_correct"`
}

type Dictionary struct {
	words []Word
}

type Trajectory struct {
	fname           string
	actions         []Action
	numInCurrentRun int
}

type WordDistribution struct {
	dictionary *Dictionary
	trajectory *Trajectory

	alpha float64
}

func readDictionary(file *os.File) *Dictionary {
	var result []Word
	var word Word

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		err := json.Unmarshal([]byte(line), &word)
		if err != nil {
			fmt.Println("Error parsing JSON! Aborting..")
			panic(err)
		}
		result = append(result, word)
	}

	return &Dictionary{result}
}

func readTrajectory(file *os.File) *Trajectory {
	var result []Action
	var action Action

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		err := json.Unmarshal([]byte(line), &action)
		if err != nil {
			fmt.Println("Error parsing JSON! Aborting..")
			panic(err)
		}
		result = append(result, action)
	}

	return &Trajectory{file.Name(), result, 0}
}

func (trajectory *Trajectory) writeTrajectory() {
	file, err := os.OpenFile(trajectory.fname, os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	writer := bufio.NewWriter(file)
	for _, action := range trajectory.actions {
		_, _ = writer.WriteString(fmt.Sprintf(`{"id": %d, "is_correct": %t}`+"\n", action.Id, action.IsCorrect))
	}
	_ = writer.Flush()
	_ = file.Close()
}

func (trajectory *Trajectory) appendTo(action Action) {
	trajectory.actions = append(trajectory.actions, action)
	trajectory.numInCurrentRun++
	// TODO: Replace with append
	trajectory.writeTrajectory()
}

func sampleFromCategoricalDistribution(probs *[]float64) int {
	numElements := len(*probs)
	cumProbs := make([]float64, numElements)
	for i, prob := range *probs {
		if i == 0 {
			cumProbs[i] = prob
		} else {
			cumProbs[i] = cumProbs[i-1] + prob
		}
	}

	t := rand.Float64()
	var elementId int
	for i, cumProb := range cumProbs {
		if cumProb > t {
			elementId = i
			break
		}
	}

	return elementId
}

func (wordDistribution *WordDistribution) computeCategoricalDistribution() *[]float64 {
	nWords := len(wordDistribution.dictionary.words)
	result := make([]float64, nWords)

	for i := 0; i < nWords; i++ {
		result[i] = wordDistribution.dictionary.words[i].Freq
	}

	for _, action := range wordDistribution.trajectory.actions {
		i := action.Id
		if action.IsCorrect {
			result[i] *= 1 - wordDistribution.alpha
		} else {
			result[i] *= 1 + wordDistribution.alpha
		}
	}

	mass := 0.0
	for i := 0; i < nWords; i++ {
		mass += result[i]
	}

	for i := 0; i < nWords; i++ {
		result[i] /= mass
	}

	return &result
}

func newWordGame(wordDistribution *WordDistribution) *WordGame {
	return &WordGame{
		buttons:          make(map[string]*widget.Button, 2),
		labels:           make(map[string]*widget.Label, 2),
		wordDistribution: wordDistribution,
	}
}

func (wordGame *WordGame) launchGameOrShowAnswer() {
	if wordGame.currentQueryLang == "" {
		wordGame.generateAnswer()
		wordGame.buttons["show"].SetText("Show solution")
	} else {
		wordGame.buttons["show"].Disable()
		wordGame.buttons["answeredCorrectly"].Enable()
		wordGame.buttons["answeredIncorrectly"].Enable()

		if wordGame.currentQueryLang == "A" {
			wordGame.labels["LangB"].SetText(wordGame.wordDistribution.dictionary.words[wordGame.currentWordId].B)
		} else {
			wordGame.labels["LangA"].SetText(wordGame.wordDistribution.dictionary.words[wordGame.currentWordId].A)
		}
	}
}

func (wordGame *WordGame) generateAnswer() {

	categoricalDistribution := wordGame.wordDistribution.computeCategoricalDistribution()

	wordId := sampleFromCategoricalDistribution(categoricalDistribution)
	word := wordGame.wordDistribution.dictionary.words[wordId]
	direction := rand.Intn(2)

	var queryLang string
	if direction == 0 {
		queryLang = "A"
		wordGame.labels["LangA"].SetText(word.A)
		wordGame.labels["LangB"].SetText("")
	} else {
		queryLang = "B"
		wordGame.labels["LangA"].SetText("")
		wordGame.labels["LangB"].SetText(word.B)
	}

	wordGame.buttons["show"].Enable()
	wordGame.buttons["answeredCorrectly"].Disable()
	wordGame.buttons["answeredIncorrectly"].Disable()

	wordGame.currentWordId = wordId
	wordGame.currentQueryLang = queryLang

}

func (wordGame *WordGame) answeredCorrectly() {
	wordGame.answered(true)
}

func (wordGame *WordGame) answeredIncorrectly() {
	wordGame.answered(false)
}

func (wordGame *WordGame) answered(correctly bool) {
	wordGame.buttons["show"].Enable()
	wordGame.buttons["answeredCorrectly"].Disable()
	wordGame.buttons["answeredIncorrectly"].Disable()

	action := Action{
		Id:        wordGame.currentWordId,
		IsCorrect: correctly,
	}
	wordGame.wordDistribution.trajectory.appendTo(action)
	wordGame.labels["numAccumulatedActions"].SetText(fmt.Sprintf("Accumulated %d actions in current run", wordGame.wordDistribution.trajectory.numInCurrentRun))

	wordGame.generateAnswer()
}

func (wordGame *WordGame) addLabel(id string, text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignCenter
	wordGame.labels[id] = label
	return label
}

func (wordGame *WordGame) addLabelWithStyle(id string, text string, align fyne.TextAlign, style fyne.TextStyle) *widget.Label {
	label := widget.NewLabelWithStyle(text, align, style)
	wordGame.labels[id] = label
	return label
}

func (wordGame *WordGame) addButton(id string, text string, action func()) *widget.Button {
	button := widget.NewButton(text, action)
	button.Importance = widget.HighImportance
	if id != "show" {
		button.Disable()
	}
	wordGame.buttons[id] = button
	return button
}

func (wordGame *WordGame) loadUI(app fyne.App) {

	programTitle := widget.NewLabelWithStyle("wordGame", fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Italic: false, Monospace: false})

	menu := &fyne.Menu{
		Label: "File",
		Items: nil,
	}
	mainMenu := fyne.NewMainMenu(menu)

	italicStyle := fyne.TextStyle{Bold: false, Italic: true, Monospace: false}
	numLoadedFromDictionary := widget.NewLabelWithStyle(fmt.Sprintf("Loaded %d frequency-annotated words", len(wordGame.wordDistribution.dictionary.words)), fyne.TextAlignCenter, italicStyle)
	numLoadedFromTrajectory := widget.NewLabelWithStyle(fmt.Sprintf("Loaded %d actions from trajectory", len(wordGame.wordDistribution.trajectory.actions)), fyne.TextAlignCenter, italicStyle)
	numAccumulatedActions := wordGame.addLabelWithStyle("numAccumulatedActions", fmt.Sprintf("Accumulated %d actions in current run", 0), fyne.TextAlignCenter, italicStyle)

	labelLangA := wordGame.addLabel("LangA", "")
	labelLangB := wordGame.addLabel("LangB", "")

	wordGame.window = app.NewWindow("wordGame")
	wordGame.window.SetMainMenu(mainMenu)
	wordGame.window.SetContent(container.NewGridWithColumns(1,
		programTitle,
		container.NewGridWithColumns(3, numLoadedFromDictionary, numLoadedFromTrajectory, numAccumulatedActions),
		container.NewGridWithColumns(2, labelLangA, labelLangB),
		container.NewGridWithColumns(3,
			wordGame.addButton("show", "Launch Game", wordGame.launchGameOrShowAnswer),
			wordGame.addButton("answeredCorrectly", "Correctly answered", wordGame.answeredCorrectly),
			wordGame.addButton("answeredIncorrectly", "Incorrectly answered", wordGame.answeredIncorrectly)),
	))
	wordGame.window.Show()
}

func playGame(wordDistribution *WordDistribution) {
	app := app.New()
	wordGame := newWordGame(wordDistribution)
	wordGame.loadUI(app)
	app.Run()
}

func main() {
	var alpha float64
	rand.Seed(time.Now().UnixNano())

	freqTableFname := flag.String("freqTableFname", "", "File name of the frequency table")
	trajectoryFname := flag.String("trajectoryFname", "", "File name of the trajectory file")
	flag.Float64Var(&alpha, "alpha", 0.1, "Decay level")
	flag.Parse()
	if *freqTableFname == "" {
		fmt.Println("Frequency table file not was not specified! Aborting..")
		os.Exit(1)
	} else if *trajectoryFname == "" {
		fmt.Println("Trajectory file not was not specified! Aborting..")
		os.Exit(1)
	} else {
		freqTableFname := *freqTableFname
		trajectoryFname := *trajectoryFname

		freqFile, freqErr := os.Open(freqTableFname)
		if errors.Is(freqErr, os.ErrNotExist) {
			fmt.Printf("Could not open %s\n", freqTableFname)
			panic(freqErr)
		}
		trajectoryFile, trajectoryErr := os.Open(trajectoryFname)
		if errors.Is(trajectoryErr, os.ErrNotExist) {
			fmt.Printf("Could not open %s\n", trajectoryFname)
			panic(trajectoryErr)
		}

		dictionary := readDictionary(freqFile)
		trajectory := readTrajectory(trajectoryFile)
		wordDistribution := WordDistribution{
			dictionary: dictionary,
			trajectory: trajectory,
			alpha:      alpha,
		}
		playGame(&wordDistribution)
	}
}
