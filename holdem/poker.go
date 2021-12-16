package holdem

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"strings"

	"github.com/tam0705/go-cfr"
)

const (
	chance  = -1
	player0 = 0
	player1 = 1
)

type Action byte

const (
	Random = 'r'
	Fold
	Call
	Raise
	AllIn
	Check  = 'c'
	Bet    = 'b'
)

type poker struct {
	Num  byte // 2~10, 11: J, 12:Q, 13:K, 14:A
	Kind byte // 1:Plums , 2:Diamond, 3:Heart, 4:Spade
}

type Cards [7]poker // 第0~1為手牌, 第2~6為公牌

type HandPotential uint8

const (
	HIGH_CARD HandPotential = 0x01
	PICTURE HandPotential = 0x02
	PAIR HandPotential = 0x04
	IN_ORDER HandPotential = 0x08
	SAME_SUIT HandPotential = 0x10
)

type PlayerAction uint8

const (
	PLAYER_ACTION_CALL  PlayerAction = 0x01 // 跟注/call
	PLAYER_ACTION_CHECK PlayerAction = 0x02 // 過牌/check
	PLAYER_ACTION_RAISE PlayerAction = 0x04 // 加注/raise
	PLAYER_ACTION_FOLD  PlayerAction = 0x08 // 放棄/fold
	PLAYER_ACTION_ALLIN PlayerAction = 0x10 // 全下/all in
)

// PokerNode implements cfr.GameTreeNode for Kuhn Poker.
type PokerNode struct {
	parent        *PokerNode
	player        int
	children      []PokerNode
	probabilities []float64
	history       string

	hand Cards
}

func NewGame() *PokerNode {
	return &PokerNode{player: chance}
}


// Close implements cfr.GameTreeNode.
func (k *PokerNode) Close() {
	k.children = nil
	k.probabilities = nil
	k.hand = nil
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
	} else if k.player == chance {
		return cfr.ChanceNodeType
	}

	return cfr.PlayerNodeType
}

func (k *PokerNode) IsTerminal() bool {
	// CHANGE THIS ONE
	return (k.history == "rrcc" || k.history == "rrcbc" ||
		k.history == "rrcbb" || k.history == "rrbc" || k.history == "rrbb")
}

// Player implements cfr.GameTreeNode.
func (k *PokerNode) Player() int {
	return k.player
}

// Utility implements cfr.GameTreeNode.
func (k *PokerNode) Utility(player int) float64 {
	cardPlayer := k.playerCard(player)
	cardOpponent := k.playerCard(1 - player)

	// By convention, terminal nodes are labeled with the player whose
	// turn it would be (i.e. not the last acting player).

	if k.history == "rrcbc" || k.history == "rrbc" {
		// Last player folded. The current player wins.
		if k.player == player {
			return 1.0
		} else {
			return -1.0
		}
	} else if k.history == "rrcc" {
		// Showdown with no bets.
		if cardPlayer > cardOpponent {
			return 1.0
		} else {
			return -1.0
		}
	}

	// Showdown with 1 bet.
	if k.history != "rrcbb" && k.history != "rrbb" {
		panic("unexpected history: " + k.history)
	}

	if cardPlayer > cardOpponent {
		return 2.0
	}

	return -2.0
}

type pokerInfoSet struct {
	history string
	card    string
}

func (p pokerInfoSet) Key() []byte {
	// CHANGE THIS ONE
	return []byte(p.history + "-" + p.card)
}

func (p pokerInfoSet) MarshalBinary() ([]byte, error) {
	return p.Key(), nil
}

func (p *pokerInfoSet) UnmarshalBinary(buf []byte) error {
	parts := strings.SplitN(string(buf), "-", 1)
	if len(parts) != 2 {
		return fmt.Errorf("invalid binary poker info set: %v", parts)
	}

	p.history = parts[0]
	p.card = parts[1]
	return nil
}

// InfoSet implements cfr.GameTreeNode.
func (k *PokerNode) InfoSet(player int) cfr.InfoSet {
	return &pokerInfoSet{
		history: k.history,
		card:    k.playerCard(player).String(),
	}
}

func (k *PokerNode) InfoSetKey(player int) []byte {
	return k.InfoSet(player).Key()
}

func (k *PokerNode) playerCard(player int) Card {
	if player == player0 {
		return k.p0Card
	}

	return k.p1Card
}

func uniformDist(n int) []float64 {
	result := make([]float64, n)
	for i := range result {
		result[i] = (1.0 + float64(i)) / float64(n*(n+1)/2)
	}
	return result
}

func (k *PokerNode) buildChildren() {
	switch len(k.history) {
	case 0:
		k.children = buildPreflop(k)
	case 1:
		k.children = buildP1Deals(k)
		k.probabilities = uniformDist(len(k.children))
	case 2:
		k.children = buildP0Deals(k)
		k.probabilities = uniformDist(len(k.children))
	case 3:
		k.children = buildRound1Children(k)
	case 4:
		k.children = buildRound2Children(k)
	case 5:
		k.children = buildFinalChildren(k)
	}
}

func buildPreflop(parent *PokerNode) []PokerNode {
	var result []PokerNode
	potentials := []HandPotential{
		PICTURE + IN_ORDER + SAME_SUIT,
		IN_ORDER + SAME_SUIT,
		PICTURE + PAIR,
		PAIR,
		PICTURE + SAME_SUIT,
		SAME_SUIT,
		PICTURE + IN_ORDER,
		IN_ORDER,
		HIGH_CARD + PICTURE,
		HIGH_CARD
	}

	for _, potential := range potentials {
		child := {
			parent: parent,
			player: chance,
			history: string(Random),
			p0Card:  card,
		}
		result = append(result, child)
	}

	return result
}

func buildPostflop(parent *PokerNode) []PokerNode {

}

func buildP0Deals(parent *PokerNode) []PokerNode {
	var result []PokerNode
	for _, card := range []Card{Jack, Queen, King} {
		child := PokerNode{
			parent:  parent,
			player:  chance,
			history: string(Random),
			p0Card:  card,
		}

		result = append(result, child)
	}

	return result
}

func buildP1Deals(parent *PokerNode) []PokerNode {
	var result []PokerNode
	for _, card := range []Card{Jack, Queen, King} {
		if card == parent.p0Card {
			continue // Both players can't be dealt the same card.
		}

		child := *parent
		child.parent = parent
		child.player = player0
		child.p1Card = card
		child.history += string([]byte{Random})
		result = append(result, child)
	}

	return result

}

func buildRound1Children(parent *PokerNode) []PokerNode {
	var result []PokerNode
	for _, choice := range []byte{Check, Bet} {
		child := *parent
		child.parent = parent
		child.player = player1
		child.history += string([]byte{choice})
		result = append(result, child)
	}
	return result
}

func buildRound2Children(parent *PokerNode) []PokerNode {
	var result []PokerNode
	for _, choice := range []byte{Check, Bet} {
		child := *parent
		child.parent = parent
		child.player = player0
		child.history += string([]byte{choice})
		result = append(result, child)
	}
	return result
}

func buildFinalChildren(parent *PokerNode) []PokerNode {
	var result []PokerNode
	if parent.history[2] == Check && parent.history[3] == Bet {
		for _, choice := range []byte{Check, Bet} {
			child := *parent
			child.parent = parent
			child.player = player1
			child.history += string([]byte{choice})
			result = append(result, child)
		}
	}

	return result
}

func init() {
	gob.Register(&pokerInfoSet{})
}
