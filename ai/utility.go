package ai

import (
	"math"
	"fmt"
	"github.com/tam0705/go-cfr/holdem"
)

func comb(n int, k int) int {
	if (n < k) {
		panic(fmt.Errorf("Error in func comb(), %d < %d", n, k))
	}
	if (n - k > k) {
		k = n - k
	}
	val := 1
	for i := k+1; i <= n; i++ {
		val *= i
	}
	for i := 2; i <= n-k; i++ {
		val /= i
	}
	return val
}

func mini(a int, b int) int {
	return int(math.Min(float64(a), float64(b)))
}

func calProb(prevOppNum, curOppNum, raiseNum int, fold, callCheck, raiseAllin float64) float32 {
	return float32(float64(comb(prevOppNum, prevOppNum - curOppNum) * comb(curOppNum, mini(curOppNum, raiseNum))) *
		math.Pow(raiseAllin, float64(mini(curOppNum, raiseNum))) * math.Pow(callCheck, float64(curOppNum - mini(curOppNum, raiseNum))) *
		math.Pow(fold, float64(prevOppNum - curOppNum)))
}

func getLastIdx(historyLength int) int {
	return 3*((historyLength-2)/3)+1
}

func getOppStrat(history string, oppType OpponentType) (int, []float32) {
	// Pre-flop has some issues
	var res []float32
	prevOppNum := 8
	isPostflop := 0
	if (len(history) > 1) {
		prevOppNum, _ = holdem.GetOpponentInfo(history[getLastIdx(len(history))])
		isPostflop = 1
	}
	fold := float64(OPPONENT_STRATEGY[oppType][isPostflop][0])
	callCheck := float64(OPPONENT_STRATEGY[oppType][isPostflop][1])
	raiseAllin := float64(OPPONENT_STRATEGY[oppType][isPostflop][2] + OPPONENT_STRATEGY[oppType][isPostflop][3])
	slice1 := []int{ 3, 2, 1, 0 }
	slice2 := []int{ 4, 3, 2, 1, 0 }
	slice3 := []int{ 5, 4, 3, 2, 1, 0 }

	for i := prevOppNum; i >= 1; i-- {
		var slice []int
		if i == 1 {
			slice = slice1
		} else if i <= 3 {
			slice = slice2
		} else if i <= 8 {
			slice = slice3
		}
		for _,j := range slice {
			res = append(res, calProb(prevOppNum, i, j, fold, callCheck, raiseAllin))
		}
	}

	return prevOppNum, res
}