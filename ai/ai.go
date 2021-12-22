package ai

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/tam0705/go-cfr"
	"github.com/tam0705/go-cfr/def"
	"github.com/tam0705/go-cfr/holdem"
	"github.com/tam0705/go-cfr/sampling"
	"github.com/tam0705/go-cfr/tree"
)

type EnemyType int

const (
	PESSIMISTIC EnemyType = iota
	NEUTRAL
	CONFIDENT
)

// Pre-determined enemy strategies
// First index: Enemy type (Pessimistic, Neutral, Confident)
// Second index: Condition (Pre-flop, post-flop, after all-in)
// Third index: Strategies (Fold, Call, Raise, All-in)
var ENEMY_STRATEGY = [3][3][4]float32{
	{
		{ 0.4, 0.4, 0.1, 0.1 },
		{ 0.3, 0.6, 0.09, 0.01 },
		{ 0.6, -1, -1, 0.4 },
	},
	{
		{ 0.05, 0.5, 0.4, 0.05 },
		{ 0.15, 0.375, 0.375, 0.1 },
		{ 0.4, -1, -1, 0.6 },
	},
	{
		{ 0.01, 0.3, 0.6, 0.09 },
		{ 0.1, 0.3, 0.3, 0.3 },
		{ 0.01, -1, -1, 0.99 },
	},
}

var poker *holdem.PokerNode
var policy *cfr.PolicyTable
var es *sampling.AverageStrategySampler
var CFR *cfr.MCCFR

var enemyType EnemyType = NEUTRAL

var hasInit bool = false

// Implementation of AI Interface
func Init(enemy EnemyType, policyFileName string) {
	rand.Seed(time.Now().UnixNano())

	policy = cfr.NewPolicyTable(cfr.DiscountParams{ LinearWeighting: true })
	poker = holdem.NewGame(policy)
	es = sampling.NewAverageStrategySampler(sampling.AverageStrategyParams{ 0.05, 1000.0, 1000000.0 })
	CFR = cfr.NewMCCFR(policy, es)
	enemyType = enemy

	if (len(policyFileName) == 0) {
		setStrategies()
	} else {
		LoadPolicy(policyFileName, true)
	}

	hasInit = true
}

func Run(nIter int) float64 {
	if (!hasInit) {
		Init(NEUTRAL, "")
	}

	expectedValue := 0.0
	for i := 1; i <= nIter; i++ {
		expectedValue += float64(CFR.Run(poker))
		if i % (nIter / 100) == 0 {
			fmt.Printf("%d iterations done.. Expected value: %.5f\n", i, expectedValue / float64(i))
		}
	}

	expectedValue /= float64(nIter)
	return expectedValue
}

func GetDecision(Informations Def.RobotInherit, OpponentPreviousAction Def.PlayerAction, Standard, Total, RaiseDiff, AllInBound float64, myHistory string) (Def.PlayerAction, float64, string) {
	return holdem.GetDecision(Informations, OpponentPreviousAction, Standard, Total, RaiseDiff, AllInBound, myHistory)
}

func GetExpectation(history string, smallBlind float64) float64 {
	if len(history) % 3 == 0 {
		history = history[:len(history)-1]
	}
	return getExpectationRecursive(poker.GetNode(history)) * smallBlind
}

func getExpectationRecursive(node cfr.GameTreeNode) float64 {
	var ev float64

	switch node.Type() {
		case cfr.TerminalNodeType:
			ev = node.Utility(1)
		default:
			child,_ := node.SampleChild()
			ev = getExpectationRecursive(child)
	}

	return ev
}

func PrintTree(maxLines int) {
	i, j, k := 0, 0, 0
	tree.VisitInfoSets(poker, func(node cfr.GameTreeNode, player int, infoSet cfr.InfoSet) {
		i++
		strat := policy.GetPolicy(node).GetStrategy()
		if strat != nil {
			if len(strat) > 0 {
				j++
				if k > maxLines {
					return
				}
				for _,s := range strat {
					if s != float32(1/len(strat)) && s != 0.0 && s != 1.0 {
						fmt.Printf("[player %d] %13s: %.5f\n", player, infoSet, strat)
						k++
						break
					} 
				}
			}
		}
	})
	fmt.Printf("There are a total of %d nodes (%d player nodes) visited.\n", i, j)
}

func PrintPolicy(maxLines int) {
	i, k := 0, 0
	policy.Iterate(func(key string, strat []float32) {
		i++
		if k > maxLines {
			return
		}
		for _,s := range strat {
			if s != float32(1/len(strat)) && s != 0.0 && s != 1.0 {
				fmt.Printf("%13s: %.5f\n", key, strat)
				k++
				break
			} 
		}
	})
	fmt.Printf("There are a total of %d keys visited.\n", i)
}

func SavePolicy(fileName string) {
	dataFile, err := os.Create(fileName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	encoder := gob.NewEncoder(dataFile)
	encoder.Encode(policy)

	dataFile.Close()
}

func LoadPolicy(fileName string, replace bool) {
	dataFile, err := os.Open(fileName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	decoder := gob.NewDecoder(dataFile)
	if replace {
		err = decoder.Decode(&policy)
	} else {
		oldPolicyTable := policy.PoliciesByKey
		var newPolicy *cfr.PolicyTable
		err = decoder.Decode(&newPolicy)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		policy.SetIter((policy.Iter() + newPolicy.Iter()) / 2)

		newPolicyTable := newPolicy.PoliciesByKey
		for key, newData := range newPolicyTable {
			_, ok := oldPolicyTable[key]
			if ok {
				oldPolicyTable[key].CombineData(newData)
			} else {
				oldPolicyTable[key] = newData
			}
		}

		policy.PoliciesByKey = oldPolicyTable
	}
	
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dataFile.Close()
}

func setStrategies() {
	// Pre-flop
	for _,potential := range holdem.HAND_POTENTIAL {
		history := string([]byte{potential})
		setStrategiesRecursive(history)
		policy.SetStrategy(history, ENEMY_STRATEGY[enemyType][0][:])
	}
}

func setStrategiesRecursive(history string) {
	policy.SetStrategy(history, ENEMY_STRATEGY[enemyType][1][:])

	if len(history) >= 10 {
		return 
	}

	// Post-flop
	for _,s := range []string{ "cc", "cr", "rc", "rr" } {
		for _,strength := range holdem.HAND_STRENGTH {
			setStrategiesRecursive(history + s + string([]byte{strength}))
		}
	}
}