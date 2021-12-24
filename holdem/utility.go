package holdem

import (
	"fmt"
	"math"
	"math/rand"

	Def "github.com/tam0705/go-cfr/def"
)

const (
	ROYAL_PROB     = 0.000032
	STRFLUSH_PROB  = 0.000279
	FOURKIND_PROB  = 0.00168
	FULLH_PROB     = 0.026
	FLUSH_PROB     = 0.0303
	STRAIGHT_PROB  = 0.0462
	THREEKIND_PROB = 0.0483
	TWOPAIR_PROB   = 0.235
	ONEPAIR_PROB   = 0.438
	HCARD_PROB     = 0.174
)

var SeatIdx Def.SeatIndex   // Seat index
var Action Def.PlayerAction // 被通知的玩家能夠行動的方式(複合的Flag) / the move availble for current player?
var Standard float64        // 現在壓注的標準
var Total float64           // 現在壓注的總額
var RaiseDiff float64       // 如果要加注，則跟到標準後，要大餘等於這個加注差值
var AllinBound float64      // 如果要Allin，則跟到標準後，只能再加這個數值

//for training purposes
const (
	SMALLBLIND_TRAIN int64 = 1
	CALL_TRAIN       int64 = 2
	ALLIN_TRAIN      int64 = 20
)

func minInt(a, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}

func maxInt(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}

func GetRaiseAmount(ConfidenceAmount, Standard, RaiseDiff, AllInBound float64, Informations Def.RobotInherit) float64 {
	var ratioToAllIn float64 = math.Min(AllInBound-Informations.BetPos, Informations.ContestMoney) / Standard
	var ratioToRaise float64 = (RaiseDiff + Standard - Informations.BetPos) / Standard
	ratioToAllIn /= Informations.SbBet
	ratioToRaise /= Informations.SbBet

	//based on the confidence amount, generate the
	//the ratio to raise, with [ratioToRaise, ratioToAllIn)
	var raiseRatio float64 = 0
	if ConfidenceAmount >= 0.4 {
		//freely generate without limit
		raiseRatio = ratioToRaise + rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.75)
	} else if ConfidenceAmount >= 0.3 {
		raiseRatio = ratioToRaise + rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.6)
	} else if ConfidenceAmount >= 0.2 {
		raiseRatio = ratioToRaise + rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.4)
	} else {
		raiseRatio = ratioToRaise + rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.25)
	}
	//convert the ratio into real amount
	raiseRatio *= Standard
	raiseRatio = math.Ceil(raiseRatio)
	if raiseRatio == (ratioToAllIn)*Standard {
		raiseRatio -= 1
	}
	raiseRatio *= Informations.SbBet
	return raiseRatio
}

//OpponentPreviousAction = set to PLAYER_ACTION_FOLD if opponent has not played this round
func GetDecision(Informations Def.RobotInherit, OpponentPreviousAction Def.PlayerAction, Standard, Total, RaiseDiff, AllInBound float64, myHistory string) (Def.PlayerAction, float64, string) {
	var currentRound byte = GetCurrentRound(Informations.Card)

	if len(myHistory) == 3*int(currentRound) {
		//check whether round repeats, if it does, clean history
		if OpponentPreviousAction == Def.PLAYER_ACTION_ALLIN {
			myHistory = myHistory[0:len(myHistory)-2] + "a" //discard the last
		} else {
			myHistory = myHistory[0:len(myHistory)-2] + "r" //discard the last
		}
	} else {
		//every new round, analyze handstrength
		if len(myHistory) == 0 {
			myHistory += checker(Informations.Card)
		} else {
			myHistory += HistoryAdd(Informations.Card)
		}

		//check whether opponent has played or not
		//since it is not possible this function is called when one of them has fold, so
		//set as fold if opponent has not moved
		if OpponentPreviousAction == Def.PLAYER_ACTION_ALLIN {
			myHistory += "a"
		} else if OpponentPreviousAction == Def.PLAYER_ACTION_RAISE {
			myHistory += "r"
		} else {
			myHistory += "c"
		}
	}

	//historyReady
	var strategyType byte
	var myStrategy []float64
	myStrategy, strategyType = GetStrategy(myHistory)

	//Fold - Call - Check - Fold - Raise, It is always possible for Folding
	var myAvailableAction [5]string = [5]string{"1", "0", "0", "0", "1"}
	//consider the available move for call/check, cant happen at same time
	if Standard == Informations.BetPos {
		myAvailableAction[1] = "1"
	} else if Standard < Informations.ContestMoney {
		myAvailableAction[2] = "1"
	}

	//consider availability for raise
	if (RaiseDiff+Standard-Informations.BetPos) < Informations.ContestMoney && (RaiseDiff+Standard) < AllInBound {
		myAvailableAction[3] = "1"
	}

	randomFloat := rand.Float64()
	myAction := Def.PLAYER_ACTION_CALL
	var myBet float64 = 0.0
	if strategyType == 2 {
		//only fold and all in
		if randomFloat < myStrategy[0] {
			myAction = Def.PLAYER_ACTION_FOLD
			myBet = 0.0
			myHistory = myHistory + "f"
		} else {
			myAction = Def.PLAYER_ACTION_ALLIN
			myBet = math.Min(AllInBound-Informations.BetPos, Informations.ContestMoney)
			myHistory = myHistory + "a"
		}
	} else if strategyType == 4 {
		//fold call/check raise and all in
		//check the availability
		if myAvailableAction[1] == "0" && myAvailableAction[2] == "0" {
			myStrategy[1] = 0.0
		}
		if myAvailableAction[3] == "0" {
			myStrategy[2] = 0.0
		}
		if myAvailableAction[4] == "0" {
			myStrategy[3] = 0.0
		}

		//if there is a bit of money left then let the call get higher
		//compared to raise
		if Standard-Informations.BetPos >= 0.5*Informations.ContestMoney {
			halfPercentage := myStrategy[2] / 2
			myStrategy[2] -= halfPercentage
			myStrategy[1] += halfPercentage
		}

		//normalize the remaining strategy
		var normalizingSum float64 = 0.0
		for a := 0; a < 4; a++ {
			normalizingSum += myStrategy[a]
		}

		if normalizingSum == 0 {
			myAction = Def.PLAYER_ACTION_FOLD
			myBet = 0.0
			myHistory += "f"
		} else {
			for a := 0; a < 4; a++ {
				myStrategy[a] /= normalizingSum
			}

			if randomFloat < myStrategy[0] {
				myAction = Def.PLAYER_ACTION_FOLD
				myBet = 0.0
				myHistory += "h"
			} else if randomFloat < myStrategy[0]+myStrategy[1] {
				if myAvailableAction[1] == "1" {
					myAction = Def.PLAYER_ACTION_CHECK
					myBet = 0.0
					myHistory += "c"
				} else {
					myAction = Def.PLAYER_ACTION_CALL
					myBet = Standard - Informations.BetPos
					myHistory += "c"
				}
			} else if randomFloat < myStrategy[0]+myStrategy[1] {
				myAction = Def.PLAYER_ACTION_RAISE
				myBet = GetRaiseAmount(myStrategy[2], Standard, RaiseDiff, AllInBound, Informations)
				myHistory += "r"
			} else {
				myAction = Def.PLAYER_ACTION_ALLIN
				myBet = math.Min(AllInBound-Informations.BetPos, Informations.ContestMoney)
				myHistory += "a"
			}
		}
	}
	return myAction, myBet, myHistory
}

func ArrangeCards(mycard Def.Cards) Def.Cards {
	for i := 0; i < 6; i++ {
		for j := 0; j < 6-i; j++ {
			if mycard[j+1].Kind == 0 && mycard[j+1].Num == 0 {
				continue
			}
			if mycard[j].Num > mycard[j+1].Num {
				var temp Def.Poker = mycard[j]
				mycard[j] = mycard[j+1]
				mycard[j+1] = temp
			} else if mycard[j].Num == mycard[j+1].Num && mycard[j].Kind == mycard[j+1].Kind {
				var temp Def.Poker = mycard[j]
				mycard[j] = mycard[j+1]
				mycard[j+1] = temp
			}
		}
	}
	return mycard
}

func HistoryAdd(mycard Def.Cards) string {
	mycard = ArrangeCards(mycard)
	var handStrength byte = 0
	var plums Def.Cards
	var plumsLength byte = 0
	var diamonds Def.Cards
	var diamondsLength byte = 0
	var hearts Def.Cards
	var heartsLength byte = 0
	var spades Def.Cards
	var spadesLength byte = 0
	var straight Def.Cards
	var straightLength byte = 0
	var fourAKind byte = 0  //only 1 possibility
	var fourAmount byte = 1 //only need the number
	var threeAKind byte = 0
	var threeSymbols [3]byte //state here the number and kind of card that is missing
	var threeAmount byte = 1
	var pairs [2][2]Def.Poker //state here the number and kind of card that is missing

	/*
		power will decide hand strength
		Royal/straight flush = 6
		4 of a kind/full house = 5
		Straight/flush = 4
		3 of a kind = 3
		2 pair = 2
		1 pair = 1
		highcard = 0
		var power int = 0;*/

	//parse the card
	for index := 0; index < 7; index++ {
		if mycard[index].Kind == 0 && mycard[index].Num == 0 {
			break
		}

		if mycard[index].Kind == 1 {
			//sort numerically
			var i byte
			for i = 0; i < plumsLength; i++ {
				if mycard[index].Num < plums[i].Num {
					break
				}
			}
			for j := plumsLength; j > i; j-- {
				plums[j] = plums[j-1]
			}
			plums[i] = mycard[index]
			plumsLength += 1
		} else if mycard[index].Kind == 2 {
			var i byte
			for i = 0; i < diamondsLength; i++ {
				if mycard[index].Num < diamonds[i].Num {
					break
				}
			}
			for j := diamondsLength; j > i; j-- {
				diamonds[j] = diamonds[j-1]
			}
			diamonds[i] = mycard[index]
			diamondsLength += 1
		} else if mycard[index].Kind == 3 {
			var i byte
			for i = 0; i < heartsLength; i++ {
				if mycard[index].Num < hearts[i].Num {
					break
				}
			}
			for j := heartsLength; j > i; j-- {
				hearts[j] = hearts[j-1]
			}
			hearts[i] = mycard[index]
			heartsLength += 1
		} else {
			var i byte
			for i = 0; i < spadesLength; i++ {
				if mycard[index].Num < spades[i].Num {
					break
				}
			}
			for j := spadesLength; j > i; j-- {
				spades[j] = spades[j-1]
			}
			spades[i] = mycard[index]
			spadesLength += 1
		}

		//take care of straight with no symbol difference
		if straightLength != 0 {
			if straightLength == 5 {
				if mycard[index].Num == straight[4].Num && mycard[index].Kind > straight[4].Kind {
					straight[4] = mycard[index]
				} else if mycard[index].Num == (straight[4].Num + 1) {
					for i := 1; i < 5; i++ {
						straight[i-1] = straight[i]
					}
					straight[4] = mycard[index]
				}
			} else {
				if mycard[index].Num == straight[straightLength-1].Num && mycard[index].Kind > straight[straightLength-1].Kind {
					straight[straightLength-1] = mycard[index]
				} else if mycard[index].Num == (straight[straightLength-1].Num + 1) {
					for i := 1; i < int(straightLength); i++ {
						straight[i-1] = straight[i]
					}
					straight[straightLength] = mycard[index]
					straightLength += 1
				} else {
					for i := 0; i < int(straightLength); i++ {
						straight[i] = Def.Poker{0, 0}
					}
					straight[0] = mycard[index]
					straightLength = 1
				}
			}
		} else {
			straight[0] = mycard[index]
			straightLength = 1
		}
		//4 of a kind, 3 of a kind or pairs
		if index > 0 {
			if mycard[index].Num == (mycard[index-1].Num) {
				//pair exist
				for i := 0; i < 2; i++ {
					if pairs[i][0].Num == 0 {
						pairs[i][0] = mycard[index-1]
						pairs[i][1] = mycard[index]
						break
					} else if pairs[i][0].Num == mycard[index].Num {
						pairs[i][0] = pairs[i][1]
						pairs[i][1] = mycard[index]
						break
					} else if i == 1 {
						pairs[0] = pairs[1]
						pairs[1][0] = mycard[index-1]
						pairs[1][1] = mycard[index]
						break
					}
				}

				//increment for three of a kind
				threeAmount += 1
				if threeAmount >= 3 {
					if threeAKind == mycard[index].Num {
						threeSymbols[0] = threeSymbols[1]
						threeSymbols[1] = threeSymbols[2]
						threeSymbols[2] = mycard[index].Kind
					} else {
						threeAKind = mycard[index].Num
						threeSymbols[0] = mycard[index-2].Kind
						threeSymbols[1] = mycard[index-1].Kind
						threeSymbols[2] = mycard[index].Kind
					}
				}

				//increment for four of a kind
				fourAmount += 1
				if fourAmount > 3 {
					fourAKind = mycard[index].Num
				}
			} else {
				//reset state
				threeAmount = 1
				fourAmount = 1
			}
		}
	}

	//analyzing handstrength
	//royal flush and straight flush
	if spadesLength == 5 || heartsLength == 5 || diamondsLength == 5 || plumsLength == 5 {
		//flush available
		//but check for royal flush/straight flush
		if spadesLength == 5 {
			for i := 1; i < 5; i++ {
				if spades[i].Num != (spades[i-1].Num + 1) {
					break
				} else if i == 4 {
					handStrength = 6
				}
			}
		}
		if heartsLength == 5 {
			for i := 1; i < 5; i++ {
				if hearts[i].Num != (hearts[i-1].Num + 1) {
					break
				} else if i == 4 {
					handStrength = 6
				}
			}
		}
		if diamondsLength == 5 {
			for i := 1; i < 5; i++ {
				if diamonds[i].Num != (diamonds[i-1].Num + 1) {
					break
				} else if i == 4 {
					handStrength = 6
				}
			}
		}
		if plumsLength == 5 {
			for i := 1; i < 5; i++ {
				if plums[i].Num != (plums[i-1].Num + 1) {
					break
				} else if i == 4 {
					handStrength = 6
				}
			}
		}

		if handStrength < 6 {
			//flush
			handStrength = 4
		}
	}

	//four of a kind, full house
	if handStrength < 5 {
		if fourAKind > 0 {
			handStrength = 5
		} else if threeAKind > 0 && pairs[0][0].Num > 0 && pairs[0][0].Num != threeAKind {
			handStrength = 5
		} else if threeAKind > 0 && pairs[1][0].Num > 0 && pairs[1][0].Num != threeAKind {
			handStrength = 5
		}
	}

	//straight
	if handStrength < 4 && straightLength == 5 {
		handStrength = 4
	}

	//3 of a kind
	if handStrength < 3 && threeAKind > 0 {
		handStrength = 3
	}

	if handStrength < 1 && pairs[0][0].Num > 0 {
		if pairs[1][0].Num > 0 {
			handStrength = 2
		} else {
			handStrength = 1
		}
	}

	/*Power encoding
	6 = a
	5 = b
	4 = c
	3 = d
	2 = e
	1 = f
	0 = g*/
	var returnString string
	if handStrength > 5 {
		returnString = "A"
	} else if handStrength > 4 {
		returnString = "B"
	} else if handStrength > 3 {
		returnString = "C"
	} else if handStrength > 2 {
		returnString = "D"
	} else if handStrength > 1 {
		returnString = "E"
	} else if handStrength > 0 {
		returnString = "F"
	} else {
		returnString = "G"
	}
	return returnString
}

func GetCurrentRound(myCard Def.Cards) byte {
	//4 = river
	//3 = turn
	//2 = flop
	//1 = preflop
	if myCard[6].Num > 0 && myCard[6].Kind > 0 {
		return byte(3)
	} else if myCard[5].Num > 0 && myCard[5].Kind > 0 {
		return byte(2)
	} else if myCard[4].Num > 0 && myCard[4].Kind > 0 {
		return byte(1)
	} else {
		return byte(0)
	}
}

func PseudoGeneratorForMyRaise(ConfidenceAmount float64, Standard, RaiseDiff, AllInBound, SbBet, ContestMoney, BetPos int64) int64 {
	var ratioToAllIn float64 = math.Min(math.Max(float64(AllInBound-BetPos), 0), float64(ContestMoney)) / float64(Standard)
	var ratioToRaise float64 = math.Max(float64(RaiseDiff+Standard-BetPos), 0) / float64(Standard)
	ratioToAllIn /= float64(SbBet)
	ratioToRaise /= float64(SbBet)

	//based on the confidence amount, generate the
	//the ratio to raise, with [ratioToRaise, ratioToAllIn)

	var raiseRatio float64 = 0
	if ratioToRaise > ratioToAllIn {
		raiseRatio = ratioToAllIn
	} else {
		if ConfidenceAmount >= 0.4 {
			//freely generate without limit
			raiseRatio = ratioToRaise + math.Max(rand.Float64()*(ratioToAllIn-ratioToRaise), 1)
		} else if ConfidenceAmount >= 0.3 {
			raiseRatio = ratioToRaise + math.Max(rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.75), 0)
		} else if ConfidenceAmount >= 0.2 {
			raiseRatio = ratioToRaise + math.Max(rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.5), 0)
		} else {
			raiseRatio = ratioToRaise + math.Max(rand.Float64()*(ratioToAllIn-ratioToRaise)*(0.25), 0)
		}
	}

	//convert the ratio into real amount
	raiseRatio *= float64(Standard)
	returnValue := int64(raiseRatio)
	returnValue -= returnValue % SbBet
	returnValue *= SbBet
	return returnValue
}

//for training purposes
func PseudoGeneratorForOpponentRaise(ConfidenceAmount float64, RaiseDiff, Standard, AllInBound int64) (returnVar int64) {
	var raiseRatio float64
	if ConfidenceAmount >= 0.6 {
		//freely generate without limit
		raiseRatio = 3.5
	} else if ConfidenceAmount >= 0.4 {
		raiseRatio = 2.5
	} else if ConfidenceAmount >= 0.3 {
		raiseRatio = 2
	} else {
		raiseRatio = 1.0
	}

	//convert the ratio into real amount
	returnVar = Standard + int64(float64(RaiseDiff)*raiseRatio)
	returnVar -= returnVar % SMALLBLIND_TRAIN
	if returnVar > AllInBound {
		returnVar = AllInBound
	}
	return returnVar
}

func PseudoGeneratorForOpponentAllIn(totalMoney int64) int64 {
	return ALLIN_TRAIN
}

func randomShuffleArray(myCard Def.Cards) Def.Cards {
	repetition := 50 + rand.Int()%51
	randomIndex1 := 0
	randomIndex2 := 0
	var temp Def.Poker
	for a := 0; a < repetition; a++ {
		randomIndex1 = rand.Int() % 7
		randomIndex2 = rand.Int() % 7
		if randomIndex1 != randomIndex2 {
			temp = myCard[randomIndex1]
			myCard[randomIndex1] = myCard[randomIndex2]
			myCard[randomIndex2] = temp
		}
	}
	return myCard
}

//Royal/straight flush = 6
//4 of a kind/full house = 5
//Straight/flush = 4
//3 of a kind = 3
//2 pair = 2
//1 pair = 1
//highcard = 0
func setRoyal() Def.Cards {
	var randomKind byte = byte(rand.Int()%4 + 1)
	var myCard Def.Cards = Def.Cards{{10, randomKind}, {11, randomKind}, {12, randomKind}, {13, randomKind}, {14, randomKind}}
	var number byte = 10
	for number >= 10 && randomKind == myCard[0].Kind {
		randomKind = byte(rand.Int()%4 + 1)
		number = byte(rand.Int()%13 + 2)
	}
	myCard[5] = Def.Poker{number, randomKind}
	var number2 byte = 10
	var randomKind2 byte = randomKind
	for (number2 >= 10 && randomKind2 == myCard[0].Kind) || (number2 >= number && randomKind2 == randomKind) {
		randomKind2 = byte(rand.Int()%4 + 1)
		number2 = byte(rand.Int()%13 + 2)
	}
	myCard[6] = Def.Poker{number2, randomKind2}
	//shuffle the card
	myCard = randomShuffleArray(myCard)
	return myCard
}

func setStraightFlush(numberOfRound int, p bool) Def.Cards {
	var randomKind byte = byte(rand.Int()%4 + 1)
	var myCard Def.Cards
	if numberOfRound > 1 {
		var startingNumber byte = byte(rand.Int()%9 + 2)
		myCard = Def.Cards{{startingNumber, randomKind}, {startingNumber + 1, randomKind}, {startingNumber + 2, randomKind}, {startingNumber + 3, randomKind}, {startingNumber + 4, randomKind}}
		var number byte = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
		for number >= startingNumber && number <= startingNumber+4 && randomKind == myCard[0].Kind {
			randomKind = byte(rand.Int()%4 + 1)
			number = byte(rand.Int()%13 + 2)
		}
		myCard[5] = Def.Poker{number, randomKind}
		var number2 byte = byte(rand.Int()%13 + 2)
		var randomKind2 byte = byte(rand.Int()%4 + 1)
		for (number2 >= startingNumber && number2 <= startingNumber+4 && randomKind == myCard[0].Kind) || (number2 >= number && randomKind2 == randomKind) {
			randomKind = byte(rand.Int()%4 + 1)
			number = byte(rand.Int()%13 + 2)
		}
		myCard[6] = Def.Poker{number2, randomKind2}
		myCard = randomShuffleArray(myCard)
	} else {
		if p {
			var startingNumber byte = byte(rand.Int()%4 + 10)
			myCard[0] = Def.Poker{startingNumber, randomKind}
			myCard[1] = Def.Poker{startingNumber + 1, randomKind}
		} else {
			var startingNumber byte = byte(rand.Int()%8 + 2)
			myCard[0] = Def.Poker{startingNumber, randomKind}
			myCard[1] = Def.Poker{startingNumber + 1, randomKind}
		}
		var generateNum byte
		var generateKind byte
		var currentSize byte = 2
		for currentSize < 7 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				generateKind = byte(rand.Int()%4 + 1)
				if currentSize == 0 {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, generateKind}
					currentSize = 1
				} else {
					var index byte = 0
					for index = 0; index < currentSize; index++ {
						if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
							break
						}
					}
					if index == currentSize {
						foundDuplicate = false
						myCard[currentSize] = Def.Poker{generateNum, generateKind}
						currentSize += 1
					}
				}
			}
		}
	}
	return myCard
}

func set4aKind() Def.Cards {
	var number byte = byte(rand.Int()%13 + 2)
	var myCard Def.Cards = Def.Cards{{number, 1}, {number, 2}, {number, 3}, {number, 4}}
	var randomKind byte = byte(rand.Int()%4 + 1)
	for number == myCard[0].Num {
		number = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
	}
	myCard[4] = Def.Poker{number, randomKind}
	var randomKind2 byte = byte(rand.Int()%4 + 1)
	var number2 byte = byte(rand.Int()%13 + 2)
	for number2 == myCard[0].Num || (number2 == myCard[4].Num && randomKind2 == myCard[4].Kind) {
		randomKind2 = byte(rand.Int()%4 + 1)
		number2 = byte(rand.Int()%13 + 2)
	}
	myCard[5] = Def.Poker{number2, randomKind2}
	var number3 byte = byte(rand.Int()%13 + 2)
	var randomKind3 byte = byte(rand.Int()%4 + 1)
	for number3 == myCard[0].Num || (number3 == myCard[4].Num && randomKind3 == myCard[4].Kind) || (number3 == myCard[5].Num && randomKind3 == myCard[5].Kind) {
		randomKind3 = byte(rand.Int()%4 + 1)
		number3 = byte(rand.Int()%13 + 2)
	}
	myCard[6] = Def.Poker{number3, randomKind3}
	//shuffle the card
	myCard = randomShuffleArray(myCard)
	return myCard
}

func setFullHouse() Def.Cards {
	var number byte = byte(rand.Int()%13 + 2)
	var number1 byte = byte(rand.Int()%13 + 2)
	for number1 == number {
		number1 = byte(rand.Int()%13 + 2)
	}
	var discardKind byte = byte(rand.Int()%4 + 1)
	var pickedKind1 byte = byte(rand.Int()%4 + 1)
	var pickedKind2 byte = byte(rand.Int()%4 + 1)
	for pickedKind1 == pickedKind2 {
		pickedKind2 = byte(rand.Int()%4 + 1)
	}
	var myCard Def.Cards = Def.Cards{{number, (discardKind + 1) % 4}, {number, (discardKind + 2) % 4}, {number, (discardKind + 3) % 4}, {number1, pickedKind1}, {number1, pickedKind2}}
	var randomKind2 byte = byte(rand.Int()%4 + 1)
	var number2 byte = byte(rand.Int()%13 + 2)
	for (number2 == number) || (number2 == number1) {
		randomKind2 = byte(rand.Int()%4 + 1)
		number2 = byte(rand.Int()%13 + 2)
	}
	myCard[5] = Def.Poker{number2, randomKind2}
	var number3 byte = byte(rand.Int()%13 + 2)
	var randomKind3 byte = byte(rand.Int()%4 + 1)
	for (number3 == number) || (number3 == number1) || (number3 == number2 && randomKind3 == randomKind2) {
		randomKind3 = byte(rand.Int()%4 + 1)
		number3 = byte(rand.Int()%13 + 2)
	}
	myCard[6] = Def.Poker{number3, randomKind3}
	//shuffle the card
	myCard = randomShuffleArray(myCard)
	return myCard
}

func setStraight(numberOfRound int, p bool) Def.Cards {
	var randomKind byte = byte(rand.Int()%4 + 1)
	var myCard Def.Cards
	if numberOfRound > 1 {
		var startingNumber byte = byte(rand.Int()%9 + 2)
		var randomKind2 byte = byte(rand.Int()%4 + 1)
		var randomKind3 byte = byte(rand.Int()%4 + 1)
		var randomKind4 byte = byte(rand.Int()%4 + 1)
		var randomKind5 byte = byte(rand.Int()%4 + 1)
		myCard = Def.Cards{{startingNumber, randomKind}, {startingNumber + 1, randomKind2}, {startingNumber + 2, randomKind3}, {startingNumber + 3, randomKind4}, {startingNumber + 4, randomKind5}}
		var number byte = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
		var duplicate bool = true
		for duplicate {
			for i := 0; i < 5; i++ {
				if number == myCard[i].Num && randomKind == myCard[i].Kind {
					break
				}
				if i == 4 {
					duplicate = false
				}
			}
			if duplicate {
				number = byte(rand.Int()%13 + 2)
				randomKind = byte(rand.Int()%4 + 1)
			}
		}
		myCard[5] = Def.Poker{number, randomKind}
		number = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
		duplicate = true
		for duplicate {
			for i := 0; i < 6; i++ {
				if number == myCard[i].Num && randomKind == myCard[i].Kind {
					break
				}
				if i == 5 {
					duplicate = false
				}
			}
			if duplicate {
				number = byte(rand.Int()%13 + 2)
				randomKind = byte(rand.Int()%4 + 1)
			}
		}
		myCard[6] = Def.Poker{number, randomKind}
		myCard = randomShuffleArray(myCard)
	} else {
		var randomKind2 byte = byte(rand.Int()%4 + 1)
		for randomKind2 == randomKind {
			randomKind2 = byte(rand.Int()%4 + 1)
		}
		if p {
			var startingNumber byte = byte(rand.Int()%4 + 10)
			myCard[0] = Def.Poker{startingNumber, randomKind}
			myCard[1] = Def.Poker{startingNumber + 1, randomKind2}
		} else {
			var startingNumber byte = byte(rand.Int()%8 + 2)
			myCard[0] = Def.Poker{startingNumber, randomKind}
			myCard[1] = Def.Poker{startingNumber + 1, randomKind2}
		}
		var generateNum byte
		var generateKind byte
		var currentSize byte = 2
		for currentSize < 7 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				generateKind = byte(rand.Int()%4 + 1)
				if currentSize == 0 {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, generateKind}
					currentSize = 1
				} else {
					var index byte = 0
					for index = 0; index < currentSize; index++ {
						if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
							break
						}
					}
					if index == currentSize {
						foundDuplicate = false
						myCard[currentSize] = Def.Poker{generateNum, generateKind}
						currentSize += 1
					}
				}
			}
		}
	}
	return myCard
}

func setFlush(numberOfRound int, p bool) Def.Cards {
	var randomKind byte = byte(rand.Int()%4 + 1)
	var myCard Def.Cards
	if numberOfRound > 1 {
		var currentSize = 0
		var generateNum byte
		for currentSize < 5 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				if currentSize == 0 {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, randomKind}
					currentSize = 1
				} else {
					var index = 0
					for index = 0; index < currentSize; index++ {
						if myCard[index].Num == generateNum {
							break
						}
					}
					if index == currentSize {
						foundDuplicate = false
						myCard[currentSize] = Def.Poker{generateNum, randomKind}
						currentSize += 1
					}
				}
			}
		}
		var number byte = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
		var duplicate bool = true
		for duplicate {
			for i := 0; i < 5; i++ {
				if number == myCard[i].Num && randomKind == myCard[i].Kind {
					break
				}
				if i == 4 {
					duplicate = false
				}
			}
			if duplicate {
				number = byte(rand.Int()%13 + 2)
				randomKind = byte(rand.Int()%4 + 1)
			}
		}
		myCard[5] = Def.Poker{number, randomKind}
		number = byte(rand.Int()%13 + 2)
		randomKind = byte(rand.Int()%4 + 1)
		duplicate = true
		for duplicate {
			for i := 0; i < 6; i++ {
				if number == myCard[i].Num && randomKind == myCard[i].Kind {
					break
				}
				if i == 5 {
					duplicate = false
				}
			}
			if duplicate {
				number = byte(rand.Int()%13 + 2)
				randomKind = byte(rand.Int()%4 + 1)
			}
		}
		myCard[6] = Def.Poker{number, randomKind}
		myCard = randomShuffleArray(myCard)
	} else {
		if p {
			var num1 byte = byte(rand.Int()%5 + 10)
			var num2 byte = byte(rand.Int()%13 + 2)
			for num2 == num1 {
				num2 = byte(rand.Int()%13 + 2)
			}
			myCard[0] = Def.Poker{num1, randomKind}
			myCard[1] = Def.Poker{num2, randomKind}
		} else {
			var num1 byte = byte(rand.Int()%9 + 2)
			var num2 byte = byte(rand.Int()%9 + 2)
			for num2 == num1 {
				num2 = byte(rand.Int()%9 + 2)
			}
			myCard[0] = Def.Poker{num1, randomKind}
			myCard[1] = Def.Poker{num2, randomKind}
		}
		var generateNum byte
		var generateKind byte
		var currentSize byte = 2
		for currentSize < 7 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				generateKind = byte(rand.Int()%4 + 1)
				var index byte = 0
				for index = 0; index < currentSize; index++ {
					if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
						break
					}
				}
				if index == currentSize {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, generateKind}
					currentSize += 1
				}
			}
		}
	}
	return myCard
}

func set3Kind() Def.Cards {
	var number byte = byte(rand.Int()%13 + 2)
	var discardKind byte = byte(rand.Int()%4 + 1)
	var myCard Def.Cards = Def.Cards{{number, (discardKind + 1) % 4}, {number, (discardKind + 2) % 4}, {number, (discardKind + 3) % 4}}
	var generateNum byte
	var generateKind byte
	var currentSize byte = 3
	for currentSize < 7 {
		var foundDuplicate = true
		for foundDuplicate {
			generateNum = byte(rand.Int()%13 + 2)
			generateKind = byte(rand.Int()%4 + 1)
			var index byte = 0
			for index = 0; index < currentSize; index++ {
				if generateNum == number || (myCard[index].Num == generateNum && myCard[index].Kind == generateKind) {
					break
				}
			}
			if index == currentSize {
				foundDuplicate = false
				myCard[currentSize] = Def.Poker{generateNum, generateKind}
				currentSize += 1
			}
		}
	}
	myCard = randomShuffleArray(myCard)
	return myCard
}

func set2Pair2() Def.Cards {
	var number byte = byte(rand.Int()%13 + 2)
	var number2 byte = byte(rand.Int()%13 + 2)
	for number == number2 {
		number2 = byte(rand.Int()%13 + 2)
	}
	var kind1 byte = byte(rand.Int()%4 + 1)
	var kind2 byte = byte(rand.Int()%4 + 1)
	for kind2 == kind1 {
		kind2 = byte(rand.Int()%4 + 1)
	}
	var myCard Def.Cards = Def.Cards{{number, kind1}, {number, kind2}}
	kind1 = byte(rand.Int()%4 + 1)
	kind2 = byte(rand.Int()%4 + 1)
	for kind2 == kind1 {
		kind2 = byte(rand.Int()%4 + 1)
	}
	myCard[2] = Def.Poker{number2, kind1}
	myCard[3] = Def.Poker{number2, kind2}
	var generateNum byte
	var generateKind byte
	var currentSize byte = 4
	for currentSize < 7 {
		var foundDuplicate = true
		for foundDuplicate {
			generateNum = byte(rand.Int()%13 + 2)
			generateKind = byte(rand.Int()%4 + 1)
			var index byte = 0
			for index = 0; index < currentSize; index++ {
				if generateNum == number || generateNum == number2 || (myCard[index].Num == generateNum && myCard[index].Kind == generateKind) {
					break
				}
			}
			if index == currentSize {
				foundDuplicate = false
				myCard[currentSize] = Def.Poker{generateNum, generateKind}
				currentSize += 1
			}
		}
	}
	myCard = randomShuffleArray(myCard)
	return myCard
}

func setPair(numberOfRound int, p bool) Def.Cards {
	var myCard Def.Cards
	if numberOfRound > 1 {
		var number byte = byte(rand.Int()%13 + 2)
		var kind1 byte = byte(rand.Int()%4 + 1)
		var kind2 byte = byte(rand.Int()%4 + 1)
		for kind2 == kind1 {
			kind2 = byte(rand.Int()%4 + 1)
		}
		myCard = Def.Cards{{number, kind1}, {number, kind2}}
		var generateNum byte
		var generateKind byte
		var currentSize byte = 2
		for currentSize < 7 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				generateKind = byte(rand.Int()%4 + 1)
				var index byte = 0
				for index = 0; index < currentSize; index++ {
					if myCard[index].Num == generateNum {
						break
					}
				}
				if index == currentSize {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, generateKind}
					currentSize += 1
				}
			}
		}
		//shuffling the card
		myCard = randomShuffleArray(myCard)
	} else {
		if p {
			var number byte = byte(rand.Int()%4 + 11)
			var kind1 byte = byte(rand.Int()%4 + 1)
			var kind2 byte = byte(rand.Int()%4 + 1)
			for kind2 == kind1 {
				kind2 = byte(rand.Int()%4 + 1)
			}
			myCard = Def.Cards{{number, kind1}, {number, kind2}}
		} else {
			var number byte = byte(rand.Int()%9 + 2)
			var kind1 byte = byte(rand.Int()%4 + 1)
			var kind2 byte = byte(rand.Int()%4 + 1)
			for kind2 == kind1 {
				kind2 = byte(rand.Int()%4 + 1)
			}
			myCard = Def.Cards{{number, kind1}, {number, kind2}}
			var generateNum byte
			var generateKind byte
			var currentSize byte = 2
			for currentSize < 7 {
				var foundDuplicate = true
				for foundDuplicate {
					generateNum = byte(rand.Int()%13 + 2)
					generateKind = byte(rand.Int()%4 + 1)
					var index byte = 0
					for index = 0; index < currentSize; index++ {
						if myCard[index].Num == generateNum {
							break
						}
					}
					if index == currentSize {
						foundDuplicate = false
						myCard[currentSize] = Def.Poker{generateNum, generateKind}
						currentSize += 1
					}
				}
			}
		}
	}
	return myCard
}

func setHighCard(numberOfRound int, p bool) Def.Cards {
	var myCard Def.Cards
	if p {
		var number byte = byte(rand.Int()%4 + 11)
		var kind byte = byte(rand.Int()%4 + 1)
		myCard[0] = Def.Poker{number, kind}
		for (math.Abs(float64(number)-float64(myCard[0].Num)) == 1) || (number == myCard[0].Num) {
			number = byte(rand.Int()%13 + 2)
		}
		for kind == myCard[0].Kind {
			kind = byte(rand.Int()%4 + 1)
		}
		myCard[1] = Def.Poker{number, kind}
	} else {
		var number byte = byte(rand.Int()%9 + 2)
		var kind byte = byte(rand.Int()%4 + 1)
		myCard[0] = Def.Poker{number, kind}
		for (math.Abs(float64(number)-float64(myCard[0].Num)) == 1) || (number == myCard[0].Num) {
			number = byte(rand.Int()%9 + 2)
		}
		for kind == myCard[0].Kind {
			kind = byte(rand.Int()%4 + 1)
		}
		myCard[1] = Def.Poker{number, kind}
	}

	var currentSize byte = 2
	if numberOfRound > 1 {
		var generateNum byte
		var generateKind byte
		for currentSize < 5 {
			var foundDuplicate = true
			for foundDuplicate {
				generateNum = byte(rand.Int()%13 + 2)
				generateKind = byte(rand.Int()%4 + 1)
				var index byte = 0
				for index = 0; index < currentSize; index++ {
					if myCard[index].Num == generateNum {
						break
					}
				}
				var temporaryCard Def.Cards = myCard
				temporaryCard[currentSize] = Def.Poker{generateNum, generateKind}
				if index == currentSize && HistoryAdd(temporaryCard) == "G" {
					foundDuplicate = false
					myCard[currentSize] = Def.Poker{generateNum, generateKind}
					currentSize += 1
				}
			}
		}
		if numberOfRound > 2 {
			for currentSize < 6 {
				var foundDuplicate = true
				for foundDuplicate {
					generateNum = byte(rand.Int()%13 + 2)
					generateKind = byte(rand.Int()%4 + 1)
					var index byte = 0
					for index = 0; index < currentSize; index++ {
						if myCard[index].Num == generateNum {
							break
						}
					}
					var temporaryCard Def.Cards = myCard
					temporaryCard[currentSize] = Def.Poker{generateNum, generateKind}
					if index == currentSize && HistoryAdd(temporaryCard) == "G" {
						foundDuplicate = false
						myCard[currentSize] = Def.Poker{generateNum, generateKind}
						currentSize += 1
					}
				}
			}
			if numberOfRound > 3 {
				for currentSize < 7 {
					var foundDuplicate = true
					for foundDuplicate {
						generateNum = byte(rand.Int()%13 + 2)
						generateKind = byte(rand.Int()%4 + 1)
						var index byte = 0
						for index = 0; index < currentSize; index++ {
							if myCard[index].Num == generateNum {
								break
							}
						}
						var temporaryCard Def.Cards = myCard
						temporaryCard[currentSize] = Def.Poker{generateNum, generateKind}
						if index == currentSize && HistoryAdd(temporaryCard) == "G" {
							foundDuplicate = false
							myCard[currentSize] = Def.Poker{generateNum, generateKind}
							currentSize += 1
						}
					}
				}
			}
		}
	}

	for currentSize < 7 {
		var generateNum byte
		var generateKind byte
		var foundDuplicate = true
		for foundDuplicate {
			generateNum = byte(rand.Int()%13 + 2)
			generateKind = byte(rand.Int()%4 + 1)
			var index byte = 0
			for index = 0; index < currentSize; index++ {
				if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
					break
				}
			}
			if index == currentSize {
				foundDuplicate = false
				myCard[currentSize] = Def.Poker{generateNum, generateKind}
				currentSize += 1
			}
		}
	}
	return myCard
}

func checker(mycard Def.Cards) string {
	sameSuit := (mycard[0].Kind == mycard[1].Kind)
	inOrder := (int(mycard[0].Num) - int(mycard[1].Num)) == 1
	if !inOrder {
		inOrder = int(mycard[0].Num)-int(mycard[1].Num) == -1
	}
	pair := mycard[0].Num == mycard[1].Num
	p := mycard[0].Num > 10 || mycard[1].Num > 10
	if sameSuit {
		if inOrder {
			if p {
				return "0"
			} else {
				return "1"
			}
		} else {
			if p {
				return "5"
			} else {
				return "6"
			}
		}
	} else if pair {
		if p {
			return "2"
		} else {
			return "3"
		}
	} else {
		if inOrder {
			if p {
				return "6"
			} else {
				return "7"
			}
		} else {
			if p {
				return "8"
			} else {
				return "9"
			}
		}
	}
}

func GenerateOpponentCard(myCard Def.Cards) (opponentCard Def.Cards) {
	//set community card
	opponentCard[2] = myCard[2]
	opponentCard[3] = myCard[3]
	opponentCard[4] = myCard[4]
	opponentCard[5] = myCard[5]
	opponentCard[6] = myCard[6]

	var generateNum byte
	var generateKind byte
	var foundDuplicate = true
	for foundDuplicate {
		generateNum = byte(rand.Int()%13 + 2)
		generateKind = byte(rand.Int()%4 + 1)
		var index byte = 2
		for index = 2; index < 7; index++ {
			if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
				break
			}
		}
		if index == 7 {
			if (generateNum != myCard[0].Num || generateKind != myCard[0].Kind) && (generateNum != myCard[1].Num || generateKind != myCard[1].Kind) {
				foundDuplicate = false
				opponentCard[1] = Def.Poker{generateNum, generateKind}
			}
		}
	}
	foundDuplicate = true
	for foundDuplicate {
		generateNum = byte(rand.Int()%13 + 2)
		generateKind = byte(rand.Int()%4 + 1)
		var index byte = 2
		for index = 1; index < 7; index++ {
			if myCard[index].Num == generateNum && myCard[index].Kind == generateKind {
				break
			}
		}
		if index == 7 {
			if (generateNum != myCard[0].Num || generateKind != myCard[0].Kind) && (generateNum != myCard[1].Num || generateKind != myCard[1].Kind) {
				foundDuplicate = false
				opponentCard[0] = Def.Poker{generateNum, generateKind}
			}
		}
	}
	return
}

func AllInWinner(lastStrength string) (winner byte) {
	var myCard Def.Cards
	var normalizingSum float64 = 0
	if lastStrength == "A" {
		normalizingSum = ROYAL_PROB + STRFLUSH_PROB
		var randFloat = rand.Float64()
		if randFloat < ROYAL_PROB/normalizingSum {
			myCard = setRoyal()
		} else {
			myCard = setStraightFlush(4, false)
		}
	} else if lastStrength == "B" {
		normalizingSum = FOURKIND_PROB + FULLH_PROB
		var randFloat = rand.Float64()
		if randFloat < FOURKIND_PROB/normalizingSum {
			myCard = set4aKind()
		} else {
			myCard = setFullHouse()
		}
	} else if lastStrength == "C" {
		normalizingSum = STRAIGHT_PROB + FLUSH_PROB
		var randFloat = rand.Float64()
		if randFloat < STRAIGHT_PROB/normalizingSum {
			myCard = setStraight(4, false)
		} else {
			myCard = setFlush(4, false)
		}
	} else if lastStrength == "D" {
		myCard = set3Kind()
	} else if lastStrength == "E" {
		myCard = set2Pair2()
	} else if lastStrength == "F" {
		myCard = setPair(5, false)
	} else if lastStrength == "G" {
		myCard = setHighCard(5, false)
	} else if lastStrength == "0" {
		myCard = setStraightFlush(1, true)
	} else if lastStrength == "1" {
		myCard = setStraightFlush(1, false)
	} else if lastStrength == "2" {
		myCard = setPair(1, true)
	} else if lastStrength == "3" {
		myCard = setPair(1, false)
	} else if lastStrength == "4" {
		myCard = setFlush(1, true)
	} else if lastStrength == "5" {
		myCard = setFlush(1, false)
	} else if lastStrength == "6" {
		myCard = setStraight(1, true)
	} else if lastStrength == "7" {
		myCard = setStraight(1, false)
	} else if lastStrength == "8" {
		myCard = setHighCard(1, true)
	} else if lastStrength == "9" {
		myCard = setHighCard(1, false)
	}

	var opponentCard Def.Cards = GenerateOpponentCard(myCard)
	myGrade := HistoryAdd(myCard)
	opponentGrade := HistoryAdd(opponentCard)
	if myGrade < opponentGrade {
		winner = 2
	} else if myGrade > opponentGrade {
		winner = 0
	} else {
		if myGrade == "G" {
			myCard = ArrangeCards(myCard)
			opponentCard = ArrangeCards(opponentCard)
			for a := 6; a > -1; a-- {
				if opponentCard[a].Num > myCard[a].Num {
					winner = 0
					return
				} else if opponentCard[a].Num < myCard[a].Num {
					winner = 2
					return
				} else {
					if opponentCard[a].Kind > myCard[a].Kind {
						winner = 0
						return
					} else if opponentCard[a].Kind < myCard[a].Kind {
						winner = 2
						return
					}
				}
			}
		} else {
			winner = 1
		}
	}
	return
}

func printit(val int64, toPrint string) {
	if val < 0 {
		fmt.Println(toPrint)
	}
}

func RewardCounter(history string, raiseHistory []float64, raiseSize int64) (Total, BetPos int64) {
	//start from TotalMoney = 10000, standard bet = 200, raise diff = 100
	var AllinBound int64 = ALLIN_TRAIN
	Total = 3 * SMALLBLIND_TRAIN
	var Standard int64 = CALL_TRAIN
	var enemyBetPos int64 = 1 * SMALLBLIND_TRAIN
	BetPos = 2 * SMALLBLIND_TRAIN
	var numRaise int64 = 0
	var raiseDiff int64 = Standard
	for i := 1; i < len(history); i++ {
		if history[i] == 'c' {
			if i%3 == 1 {
				//enemy
				Total += maxInt(Standard-enemyBetPos, 0)
				enemyBetPos = Standard
			} else {
				//mine
				Total += maxInt(Standard-BetPos, 0)
				BetPos = Standard
			}
		} else if history[i] == 'r' {
			if i%3 == 1 {
				//enemy
				var enemyRaise int64
				if raiseHistory[numRaise] >= 0.6 {
					//freely generate without limit
					enemyRaise = Standard + raiseDiff*2
				} else if raiseHistory[numRaise] >= 0.4 {
					enemyRaise = Standard + int64(float64(raiseDiff)*1.75)
				} else if raiseHistory[numRaise] >= 0.3 {
					enemyRaise = Standard + int64(float64(raiseDiff)*1.25)
				} else {
					enemyRaise = Standard + raiseDiff
				}
				raiseDiff = enemyRaise - Standard
				Total += enemyRaise
				Standard = enemyRaise
				enemyBetPos = enemyRaise
				numRaise += 1
			} else {
				//mine
				var myRaise int64
				if raiseHistory[numRaise] >= 0.6 {
					//freely generate without limit
					myRaise = Standard + raiseDiff*2
				} else if raiseHistory[numRaise] >= 0.4 {
					myRaise = Standard + int64(float64(raiseDiff)*1.75)
				} else if raiseHistory[numRaise] >= 0.3 {
					myRaise = Standard + int64(float64(raiseDiff)*1.25)
				} else {
					myRaise = Standard + raiseDiff
				}
				raiseDiff = myRaise - Standard
				Total += myRaise
				Standard = myRaise
				enemyBetPos = myRaise
				numRaise += 1
			}
		} else if history[i] == 'a' {
			if i%3 == 1 {
				//enemy
				Total += AllinBound
				enemyBetPos += AllinBound
				Standard = minInt(enemyBetPos, Standard)
			} else {
				//mine
				Total += AllinBound
				BetPos += AllinBound
				Standard = minInt(BetPos, Standard)
			}
		}
	}
	return
}