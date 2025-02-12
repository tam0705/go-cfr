package holdem

import (
	"encoding/gob"
	"fmt"
	"math/rand"

	"github.com/tam0705/go-cfr"
)

const (
	NODE_CHANCE   = -1
	NODE_AI       = 0
	NODE_OPPONENT = 1
)

const (
	ACTION_FOLD   byte = 'f'
	ACTION_CALL   byte = 'c'
	ACTION_RAISE  byte = 'r'
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
	'C', // straight || Flush
	'D', // threeOfAKind
	'E', // twoPairs
	'F', // onePair
	'G', // highCard
}

// Encoding for remaining num of opponents + num of raises (H to u, some skipped)
var ENC_OPPONENT = [2][]byte{
	{'I', 'J', 'K', 'L', '!'}, // 8-4 opponents
	{'k', 'l', 'm', '^'},      // 3-1 opponent(s)
}
var LOWER_BOUND = [2]int{4, 1}
var UPPER_BOUND = [2]int{8, 3}

var PROB_PREFLOP = [10]float64{0.006, 0.012, 0.024, 0.054, 0.127, 0.109, 0.024, 0.048, 0.308, 0.288}

//new feature
var PROB_FLOP_DETAIL = [10][7]float64{
	{0.001258, 0.023501, 0.120919, 0.043026, 0.219528, 0.419889, 0.171879},
	{0.001560, 0.033909, 0.127559, 0.056606, 0.238955, 0.401088, 0.140323},
	{0.000109, 0.120814, 0.029908, 0.099367, 0.446634, 0.303168, 0.000000},
	{0.000162, 0.120814, 0.034382, 0.099087, 0.445545, 0.300010, 0.000000},
	{0.000355, 0.033909, 0.091614, 0.057559, 0.242643, 0.420957, 0.152963},
	{0.000674, 0.033909, 0.103119, 0.057318, 0.241712, 0.414887, 0.148381},
	{0.000128, 0.033909, 0.073241, 0.058264, 0.246075, 0.430919, 0.157463},
	{0.000165, 0.033909, 0.087417, 0.057996, 0.245031, 0.423656, 0.151825},
	{0.000132, 0.033909, 0.048020, 0.058972, 0.248822, 0.444644, 0.165501},
	{0.000161, 0.033909, 0.060566, 0.058725, 0.247864, 0.438231, 0.160544}}

var PROB_TURN_DETAIL = [7][7]float64{
	{1, 0, 0, 0, 0, 0, 0},
	{0, 1, 0, 0, 0, 0, 0},
	{0.006720, 0.000000, 0.993280, 0.000000, 0.000000, 0.000000, 0.000000},
	{0.000055, 0.333950, 0.012354, 0.653642, 0.000000, 0.000000, 0.000000},
	{0.000073, 0.167438, 0.014919, 0.000000, 0.817570, 0.000000, 0.000000},
	{0.000199, 0.025902, 0.045818, 0.065364, 0.366782, 0.495935, 0.000000},
	{0.000280, 0.000000, 0.092060, 0.013767, 0.082277, 0.459604, 0.352012}}

var PROB_RIVER_DETAIL = [7][7]float64{
	{1, 0, 0, 0, 0, 0, 0},
	{0, 1, 0, 0, 0, 0, 0},
	{0.003311, 0.000000, 0.996689, 0.000000, 0.000000, 0.000000, 0.000000},
	{0.000064, 0.217391, 0.014516, 0.768029, 0.000000, 0.000000, 0.000000},
	{0.000083, 0.088274, 0.016999, 0.000000, 0.894644, 0.000000, 0.000000},
	{0.000120, 0.000000, 0.040593, 0.043307, 0.258815, 0.657164, 0.000000},
	{0.000134, 0.000000, 0.073151, 0.013767, 0.082277, 0.385464, 0.541251}}

var PROB_POSTFLOP = [4][7]float64{
	{0.0008, 0.0017, 0.0059, 0.0211, 0.0475, 0.4226, 0.5012},
	{0.000091, 0.00887, 0.0279, 0.036, 0.1244, 0.478, 0.325},
	{0.0003, 0.0277, 0.0765, 0.0483, 0.2350, 0.4380, 0.1740},
}

// PokerNode implements cfr.GameTreeNode for Kuhn Poker.
type PokerNode struct {
	parent        *PokerNode
	player        int
	children      []PokerNode
	probabilities []float64
	history       string

	handStrength string
	opponentNum  int
}

var pokerGame *PokerNode
var policy *cfr.PolicyTable

func NewGame(p *cfr.PolicyTable) *PokerNode {
	policy = p
	pokerGame = &PokerNode{player: NODE_CHANCE, history: ""}
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
	if k.probabilities == nil {
		return 0.0
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
	fmt.Println("Finding node.. ", k.history, history)
	if len(k.history) > len(history) {
		fmt.Println("NODE 1")
		return nil
	}
	if k.history != history[:len(k.history)] {
		fmt.Println("NODE 2")
		return nil
	}
	if k.history == history {
		return k
	}
	if k.IsTerminal() {
		return k
	}

	if k.children == nil {
		k.buildChildren()
	}

	for _, child := range k.children {
		result := child.GetNode(history)
		if result != nil {
			return result
		}
	}
	fmt.Println("NODE 3")
	return nil
}

func GetStrategy(history string) []float64 {
	policyData, ok := policy.GetPolicyByKey(history)

	if !ok {
		policy.SetStrategy(history, uniformDist32(pokerGame.GetNode(history).NumChildren()))
		policyData, ok = policy.GetPolicyByKey(history)
	}

	strat := policyData.GetStrategy()
	strat64 := make([]float64, len(strat))
	for i, s := range strat {
		strat64[i] = float64(s)
	}
	return strat64
}

func (k *PokerNode) IsTerminal() bool {
	if len(k.history) <= 1 {
		return false
	}
	return (k.history[len(k.history)-1] == 'f' ||
		k.history[len(k.history)-1] == 'a' || k.opponentNum == 0)
}

// Player implements cfr.GameTreeNode.
func (k *PokerNode) Player() int {
	return k.player
}

// Utility implements cfr.GameTreeNode.
func (k *PokerNode) Utility(player int) float64 {
	// Get arguments required to get total and betPos..
	raiseArr := make([]float64, 0)
	for i, b := range k.history {
		if b == 'r' {
			policyData, _ := policy.GetPolicyByKey(k.history[:i])
			raiseArr = append(raiseArr, float64((policyData.GetStrategy())[2]))
		}
	}
	total, betPos := RewardCounter(k.history, raiseArr, int64(len(raiseArr)))

	if k.opponentNum == 0 {
		// If all opponents folded
		if len(k.history) < 13 {
			return float64(total - betPos)
		}

		// If showdown is reached
		myStrength := k.handStrength[3]
		diff := 0
		for _, s := range k.handStrength[4:] {
			if myStrength < byte(s) {
				diff = 1
			} else if myStrength > byte(s) {
				diff = -1
				break
			}
		}
		if diff > 0 {
			return float64(total - betPos)
		} else if diff < 0 {
			return float64(-betPos)
		}
	} else if k.history[len(k.history)-1] == 'f' {
		// If AI folded
		return float64(-betPos)
	} else if k.history[len(k.history)-1] == 'a' {
		// If AI did all-in
		lastStrength := k.handStrength[len(k.handStrength)-1]
		win := AllInWinner(string([]byte{lastStrength}), getRandOppNum(k.opponentNum))
		if win == byte(2) {
			return float64(total - betPos)
		} else if win == byte(0) {
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

func uniformDist32(n int) []float32 {
	result := make([]float32, n)
	num := float32(1.0) / float32(n)
	for i := range result {
		result[i] = num
	}
	return result
}

func getRandOppNum(n int) int {
	i, num := 0, 0
	for i, num = range LOWER_BOUND {
		if n == num {
			break
		}
	}
	return rand.Intn(UPPER_BOUND[i]-num) + num
}

func (k *PokerNode) buildChildren() {
	switch len(k.history) {
	case 0:
		k.children = buildPreflop(k)
		k.probabilities = PROB_PREFLOP[:]
	case 1, 4, 7, 10:
		k.children = buildOpponentDeals(k)
	case 2, 5, 8, 11:
		k.children = buildAIDeals(k)
	case 3, 6, 9:
		if k.history[len(k.history)-1] != ACTION_FOLD &&
			k.history[len(k.history)-1] != ACTION_ALL_IN {
			k.children = buildPostflop(k, false)
			//k.probabilities = PROB_POSTFLOP[int(len(k.history)/3-1)][:]
			//NEW FEATURE
			if len(k.history) == 3 {
				k.probabilities = PROB_FLOP_DETAIL[k.history[0]-'0'][:]
			} else if len(k.history) == 6 {
				k.probabilities = PROB_TURN_DETAIL[k.history[3]-'A'][:]
			} else {
				k.probabilities = PROB_RIVER_DETAIL[k.history[6]-'A'][:]
			}
		}
	default:
		if k.history[len(k.history)-1] != ACTION_FOLD &&
			k.history[len(k.history)-1] != ACTION_ALL_IN && k.opponentNum > 0 {
			k.children = buildPostflop(k, true)
			k.probabilities = PROB_POSTFLOP[2][:]
		}
	}
}

func buildPreflop(parent *PokerNode) []PokerNode {
	var result []PokerNode

	for _, potential := range HAND_POTENTIAL {
		child := PokerNode{
			parent:       parent,
			player:       NODE_OPPONENT,
			history:      string([]byte{potential}),
			handStrength: string([]byte{potential}),
			opponentNum:  LOWER_BOUND[0],
		}
		result = append(result, child)
	}

	return result
}

func buildPostflop(parent *PokerNode, isShowdown bool) []PokerNode {
	var result []PokerNode

	for _, strength := range HAND_STRENGTH {
		// How bout from hand potential to hand strength?
		if !isShowdown {
			if parent.handStrength[len(parent.handStrength)-1] > '9' &&
				strength > parent.handStrength[len(parent.handStrength)-1] {
				continue
			}
		}
		child := *parent
		child.parent = parent
		child.player = NODE_OPPONENT
		child.history += string([]byte{strength})
		child.handStrength += string([]byte{strength})
		if isShowdown {
			child.player = NODE_CHANCE
			child.opponentNum--
		}
		result = append(result, child)
	}

	return result
}

func buildOpponentDeals(parent *PokerNode) []PokerNode {
	// Build nodes of opponents dealing
	var result []PokerNode

	for i, slice := range ENC_OPPONENT {
		if LOWER_BOUND[i] > parent.opponentNum {
			continue
		}
		for _, action := range slice {
			child := *parent
			child.parent = parent
			child.player = NODE_AI
			child.history += string([]byte{action})
			child.opponentNum = LOWER_BOUND[i]
			result = append(result, child)
		}
	}

	return result
}

func buildAIDeals(parent *PokerNode) []PokerNode {
	// Build nodes of AI dealing
	var result []PokerNode

	for _, action := range []byte{ACTION_FOLD, ACTION_CALL, ACTION_RAISE, ACTION_ALL_IN} {
		child := *parent
		child.parent = parent
		child.player = NODE_CHANCE
		child.history += string([]byte{action})
		result = append(result, child)
	}

	return result
}

func GetOpponentInfo(char byte) (int, int) {
	for i, slice := range ENC_OPPONENT {
		for _, e := range slice {
			if char == e {
				return LOWER_BOUND[i], i
			}
		}
	}
	return -1, -1
}

func init() {
	gob.Register(&pokerInfoSet{})
}
