package main
import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"
)
import "flag"

type Word struct {
	A string `json:"a"`
	B string `json:"b"`
	Freq float64 `json:"freq"`
}

type Action struct {
	Id uint32 `json:"id"`
	IsCorrect bool `json:"is_correct"`
}

type Dictionary struct {
	words []Word
}

type Trajectory struct {
	actions []Action
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

	return &Dictionary {result }
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

	return &Trajectory{ result }
}

func sampleFromCategoricalDistribution(probs *[]float64) int {
	nWords := len(*probs)
	cumProbs := make([]float64, nWords)
	for i, prob := range *probs {
		if i == 0 {
			cumProbs[i] = prob
		} else {
			cumProbs[i] = cumProbs[i-1]	+ prob
		}
	}

	t := rand.Float64()
	var wordId int
	for i, cumProb := range cumProbs {
		if cumProb > t {
			wordId = i
			break
		}
	}

	return wordId
}

func computeCategoricalDistribution(dictionary *Dictionary, trajectory *Trajectory, alpha float64) *[]float64 {
	nWords := len(dictionary.words)
	result := make([]float64, nWords)

	for i := 0; i < nWords; i++ {
		result[i] = dictionary.words[i].Freq
	}

	for _, action := range trajectory.actions {
		i := action.Id
		if action.IsCorrect {
			result[i] *= 1 - alpha
		} else {
			result[i] *= 1 + alpha
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

func playGame(dictionary *Dictionary, trajectory *Trajectory, alpha float64) {

	categoricalDistribution := computeCategoricalDistribution(dictionary, trajectory, alpha)

	stopRequested := false
	iter := 0
	for !stopRequested {
		wordId := sampleFromCategoricalDistribution(categoricalDistribution)
		word := dictionary.words[wordId]
		fmt.Println(word.A)
		stopRequested = true
		iter++
	}
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
		playGame(dictionary, trajectory, alpha)
	}
}
