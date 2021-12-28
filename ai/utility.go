package ai

import (
	"math"
	"fmt"
	"github.com/tam0705/go-cfr/holdem"
)

var calculatedCombs map[string]float32 = make(map[string]float32)
var calculatedStrats map[string][]float32 = make(map[string][]float32)

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

func calProb(prevOppNum, curOppNum, raiseNum int, isPostflop int) float32 {
	key := string([]byte{ byte(prevOppNum), byte(curOppNum), byte(raiseNum) })
	res, ok := calculatedCombs[key]

	if !ok {
		fold := float64(OPPONENT_STRATEGY[opponentType][isPostflop][0])
		callCheck := float64(OPPONENT_STRATEGY[opponentType][isPostflop][1])
		raiseAllin := float64(OPPONENT_STRATEGY[opponentType][isPostflop][2] + OPPONENT_STRATEGY[opponentType][isPostflop][3])
		
		res = float32(float64(comb(prevOppNum, prevOppNum - curOppNum) * comb(curOppNum, mini(curOppNum, raiseNum))) *
			math.Pow(raiseAllin, float64(mini(curOppNum, raiseNum))) * math.Pow(callCheck, float64(curOppNum - mini(curOppNum, raiseNum))) *
			math.Pow(fold, float64(prevOppNum - curOppNum)))

		calculatedCombs[key] = res
	}

	return res
}

func getLastOpponentEncoding(history string) byte {
	return history[3*int((len(history)-2)/3)+1]
}

func getLastStrength(history string) byte {
	return history[3*int((len(history)-1)/3)]
}

func getOppStrat(history string) (int, []float32) {
	// Pre-flop has some issues
	prevOppNum := 8
	isPostflop := 0
	if (len(history) > 1) {
		prevOppNum, _ = holdem.GetOpponentInfo(getLastOpponentEncoding(history))
		isPostflop = 1
	}
	
	key := string([]byte{ byte(prevOppNum), byte(isPostflop) })
	res, ok := calculatedStrats[key]
	if !ok {
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
				res = append(res, calProb(prevOppNum, i, j, isPostflop))
			}
		}

		calculatedStrats[key] = res
	}

	return prevOppNum, res
}