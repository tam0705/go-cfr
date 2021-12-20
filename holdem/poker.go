package holdem

import (
	"encoding/gob"
	"fmt"
	"math/rand"

	"github.com/tam0705/go-cfr"
)

const (
	NODE_CHANCE  = -1
	NODE_PLAYER0 = 0
	NODE_PLAYER1 = 1
)

const (
	ACTION_FOLD byte = 'f'
	ACTION_CALL byte = 'c'
	ACTION_RAISE byte = 'r'
	ACTION_ALL_IN byte = 'a'
)

var HAND_POTENTIAL = [10]byte{
	'0', // sameSuit + inOrder + picture
	'1', // sameSuit + inOrder
	'2', // pair + picture
	'3', // pair
	'4', // sameSuit + picture
	'5', // sameSuit
	'6', // inOrder + picture
	'7', // inOrder
	'8', // picture + highCard
	'9', // highCard
}

var HAND_STRENGTH = [7]byte{
	'A', // royalFlush || straightFlush
	'B', // fullHouse || fourOfAKind
	'C', // straightFlush
	'D', // threeOfAKind
	'E', // twoPairs
	'F', // onePair
	'G', // highCard
}


var PROB_PREFLOP = [10]float64{ 0.006, 0.012, 0.024, 0.054, 0.127, 0.109, 0.024, 0.048, 0.308, 0.288 }

var PROB_POSTFLOP = [4][7]float64{
	{ 0.0008, 0.0017, 0.0059, 0.0211, 0.0475, 0.4226, 0.5012 },
	{ 0.000091, 0.00887, 0.0279, 0.036, 0.1244, 0.478, 0.325 },
	{ 0.0003, 0.0277, 0.0765, 0.0483, 0.2350, 0.4380, 0.1740 },
	{ 0.0003, 0.0277, 0.0765, 0.0483, 0.2350, 0.4380, 0.1740 },
}

var PlayerNum int = 2

// PokerNode implements cfr.GameTreeNode for Kuhn Poker.
type PokerNode struct {
	parent        *PokerNode
	player        int
	children      []PokerNode
	probabilities []float64
	history       string
	handStrength string
}

var pokerGame *PokerNode
var policy *cfr.PolicyTable

func NewGame(p *cfr.PolicyTable) *PokerNode {
	policy = p
	pokerGame = &PokerNode{player: NODE_CHANCE}
	return pokerGame
}

// String implements fmt.Stringer.
func (k PokerNode) String() string {
	return fmt.Sprintf("Player %v's turn. History: %13s HandStrength: %4s",
		k.player, k.history, k.handStrength)
}

// Close implements cfr.GameTreeNode.
func (k *PokerNode) Close() {
	k.children = nil
	k.probabilities = nil
}

// NumChildren implements cfr.GameTreeNode.
func (k *PokerNode) NumChildren() int {
	if k.children == nil {
		k.buildChildren()
	}

	return len(k.children)
}

// GetChild implements cfr.GameTreeNode.
func (k *PokerNode) GetChild(i int) cfr.GameTreeNode {
	if k.children == nil {
		k.buildChildren()
	}

	return &k.children[i]
}

// Parent implements cfr.GameTreeNode.
func (k *PokerNode) Parent() cfr.GameTreeNode {
	return k.parent
}

// Get functions
func (k PokerNode) GetProb() []float64 {
	return k.probabilities
}

// GetChildProbability implements cfr.GameTreeNode.
func (k *PokerNode) GetChildProbability(i int) float64 {
	if k.children == nil {
		k.buildChildren()
	}

	return k.probabilities[i]
}

// SampleChild implements cfr.GameTreeNode.
func (k *PokerNode) SampleChild() (cfr.GameTreeNode, float64) {
	i := rand.Intn(k.NumChildren())
	return k.GetChild(i), k.GetChildProbability(i)
}

// Type implements cfr.GameTreeNode.
func (k *PokerNode) Type() cfr.NodeType {
	if k.IsTerminal() {
		return cfr.TerminalNodeType
	} else if k.player == NODE_CHANCE {
		return cfr.ChanceNodeType
	}

	return cfr.PlayerNodeType
}

// cfr.GameTreeNode implementation
func (k *PokerNode) GetNode(history string) cfr.GameTreeNode {
	// Recursive method
	if (len(k.history) > len(history)) { return nil }
	if (k.history != history[:len(k.history)]) { return nil }
	if k.history == history { return k }

	if k.children == nil {
		k.buildChildren()
	}
	
	for _, child := range k.children {
		result := child.GetNode(history)
		if result != nil { return result }
	}

	return nil
}

func GetStrategy(history string) ([]float64, byte) {
	policyData, ok := policy.GetPolicyByKey(history)

	if (!ok) {
		policyData.SetStrategy(make([]float32, pokerGame.GetNode(history).NumChildren()))
	}

	strat := policyData.GetStrategy()
	strat64 := make([]float64, len(strat))
	for i,s := range strat {
		strat64[i] = float64(s)
	}
	return strat64, byte(len(strat))
}

func (k *PokerNode) IsTerminal() bool {
	// Only valid for two players
	if len(k.history) < 1 {
		return false
	}
	if len(k.history) >= 1 {
		if k.history[len(k.history) - 1] == 'f' {
			return true
		}
	}
	if len(k.history) >= 2 {
		if (k.history[len(k.history) - 2:] == "aa" || k.history[len(k.history) - 2:] == "ca" ||
		k.history[len(k.history) - 2:] == "ra") {
			return true
		}
	}
	return (len(k.history) == 12)
}

// Player implements cfr.GameTreeNode.
func (k *PokerNode) Player() int {
	return k.player
}

// Utility implements cfr.GameTreeNode.
func (k *PokerNode) Utility(player int) float64 {
	// Get arguments required to get total and betPos..
	raiseArr := make([]float64, 0)
	for i,b := range k.history {
		if (b == 'r') {
			policyData, _ := policy.GetPolicyByKey(k.history[:i])
			raiseArr = append(raiseArr, float64((policyData.GetStrategy())[1]))
		}
	}

	total, betPos := RewardCounter(k.history, raiseArr, int64(len(raiseArr)))
	printit(total, "Total negative!")
	printit(betPos, "BetPos negative!")

	if len(k.history) == 13 {
		// If showdown is reached
		diff := int(k.history[len(k.history) - 1] - k.history[len(k.history) - 3])
		if (diff > 0) {
			return float64(total - betPos)
		} else if (diff < 0) {
			return float64(-betPos)
		}
	} else if k.history[len(k.history) - 1] == 'f' {
		// If a player folds
		if (k.player == player) {
			return float64(total - betPos)
		} else {
			return float64(-betPos)
		}
	} else if (k.history[len(k.history) - 2:] == "aa" ||
		k.history[len(k.history) - 2:] == "ca" ||
		k.history[len(k.history) - 2:] == "ra") {
		// If both players do all-in
		lastStrength := k.handStrength[len(k.handStrength) - 1]
		if (len(k.handStrength) >= 2) {
			if (k.history[len(k.history) - 2:] != "aa") {
				lastStrength = k.handStrength[len(k.handStrength) - 2]
			}
		}
		win := AllInWinner(string([]byte{lastStrength}))
		if (win == byte(2)) {
			return float64(total - betPos)
		} else if (win == byte(0)) {
			return float64(-betPos)
		}
	}

	return 0.0
}

type pokerInfoSet struct {
	history string
}

func (p pokerInfoSet) Key() []byte {
	return []byte(p.history)
}

func (p pokerInfoSet) MarshalBinary() ([]byte, error) {
	return p.Key(), nil
}

func (p *pokerInfoSet) UnmarshalBinary(buf []byte) error {
	p.history = string(buf)
	return nil
}

// InfoSet implements cfr.GameTreeNode.
func (k *PokerNode) InfoSet(player int) cfr.InfoSet {
	return &pokerInfoSet{
		history: k.history,
	}
}

func (k *PokerNode) InfoSetKey(player int) []byte {
	return k.InfoSet(player).Key()
}

func uniformDist(n int) []float64 {
	result := make([]float64, n)
	num := 1.0 / float64(n)
	for i := range result {
		result[i] = num
	}
	return result
}

func (k *PokerNode) buildChildren() {
	switch len(k.history) {
	case 0:
		k.children = buildPreflop(k)
		k.probabilities = PROB_PREFLOP[:]
	case 1, 4, 7, 10:
		if k.history[len(k.history)-1] == ACTION_ALL_IN {
			k.children = buildPlayerAllin(k, 1)	
		} else if k.history[len(k.history)-1] != ACTION_FOLD {
			k.children = buildPlayerDeals(k, 1)
		}
	case 2, 5, 8, 11:
		if k.history[len(k.history)-1] == ACTION_ALL_IN {
			k.children = buildPlayerAllin(k, 0)	
		} else if k.history[len(k.history)-1] != ACTION_FOLD {
			k.children = buildPlayerDeals(k, 0)
		}
	case 3, 6, 9, 12:
		if (k.history[len(k.history)-1] != ACTION_FOLD &&
		k.history[len(k.history)-1] != ACTION_ALL_IN) {
			k.children = buildPostflop(k)
			k.probabilities = PROB_POSTFLOP[int(len(k.history)/3-1)][:]
		}
	}
}

func buildPreflop(parent *PokerNode) []PokerNode {
	var result []PokerNode

	for _, potential := range HAND_POTENTIAL {
		child := PokerNode{
			parent: parent,
			player: NODE_PLAYER1,
			history: string([]byte{potential}),
			handStrength: string([]byte{potential}),
		}
		result = append(result, child)
	}

	return result
}

func buildPostflop(parent *PokerNode) []PokerNode {
	var result []PokerNode

	for _, strength := range HAND_STRENGTH {
		child := *parent
		child.parent = parent
		child.player = NODE_PLAYER1
		child.history += string([]byte{strength})
		child.handStrength += string([]byte{strength})
		result = append(result, child)
	}

	return result
}

func buildPlayerDeals(parent *PokerNode, player int) []PokerNode {
	// Player player deals, build next player's node
	var result []PokerNode

	for _, action := range []byte{ ACTION_FOLD, ACTION_CALL, ACTION_RAISE, ACTION_ALL_IN } {
		child := *parent
		child.parent = parent
		child.player = player - 1
		child.history += string([]byte{action})
		result = append(result, child)
	}

	return result
}

func buildPlayerAllin(parent *PokerNode, player int) []PokerNode {
	// Player player all in, build next player's node
	// Only applies for the first all in player
	var result []PokerNode

	for _, action := range []byte{ ACTION_FOLD, ACTION_ALL_IN } {
		child := *parent
		child.parent = parent
		child.player = player - 1
		child.history += string([]byte{action})
		result = append(result, child)
	}

	return result
}

func init() {
	gob.Register(&pokerInfoSet{})
}
