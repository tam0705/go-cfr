package ai

import (
	"fmt"
	"math"

	"github.com/tam0705/go-cfr/holdem"
)

var RAISE_NUMS = [2][]int{
	{4, 3, 2, 1, 0},
	{3, 2, 1, 0},
}

var calculatedCombs = map[string]float32{}
var calculatedFolds = map[string][]float32{}
var calculatedStrats = map[string][]float32{}

func comb(n int, k int) int {
	if n < k {
		panic(fmt.Errorf("Error in func comb(), %d < %d", n, k))
	}
	if n-k > k {
		k = n - k
	}
	val := 1
	for i := k + 1; i <= n; i++ {
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

func calFoldProb(oppNum, isPostflop int) []float32 {
	key := string([]byte{byte(oppNum), byte(isPostflop)})
	res, ok := calculatedFolds[key]

	if !ok {
		fold := float64(OPPONENT_STRATEGY[opponentType][isPostflop][0])
		res = append(res, 0.0)
		if oppNum == 4 {
			for i := 0; i < 4; i++ {
				res[0] += float32(comb(8, i)) * float32(math.Pow(fold, float64(i))*math.Pow(1.0-fold, float64(8-i)))
			}
		}
		res = append(res, 1-res[0])
		calculatedFolds[key] = res
	}

	return res
}

func calProb(oppNum, raiseNum, isPostflop int) float32 {
	key := string([]byte{byte(oppNum), byte(raiseNum), byte(isPostflop)})
	res, ok := calculatedCombs[key]

	if !ok {
		foldCall := float64(OPPONENT_STRATEGY[opponentType][isPostflop][0] + OPPONENT_STRATEGY[opponentType][isPostflop][1])
		raiseAllin := float64(OPPONENT_STRATEGY[opponentType][isPostflop][2] + OPPONENT_STRATEGY[opponentType][isPostflop][3])
		if oppNum < 3 {
			oppNum = 3
		} else if oppNum <= 8 {
			oppNum = 4
		}
		res = float32(comb(oppNum, raiseNum)) * float32(math.Pow(raiseAllin, float64(raiseNum))*math.Pow(foldCall, float64(oppNum-raiseNum)))

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
	prevOppNum := 4
	boundIndex := 0
	isPostflop := 0
	if len(history) > 1 {
		prevOppNum, boundIndex = holdem.GetOpponentInfo(getLastOpponentEncoding(history))
		isPostflop = 1
	}

	key := string([]byte{byte(prevOppNum), byte(isPostflop)})
	res, ok := calculatedStrats[key]
	if !ok {
		folds := calFoldProb(prevOppNum, isPostflop)
		for i := boundIndex; i < len(RAISE_NUMS); i++ {
			for _, j := range RAISE_NUMS[i] {
				res = append(res, calProb(holdem.LOWER_BOUND[i], j, isPostflop)*folds[i])
			}
		}

		calculatedStrats[key] = res
	}

	return prevOppNum, res
}

func free() {
	calculatedCombs = nil
	calculatedFolds = nil
	calculatedStrats = nil
}
