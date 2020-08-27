package cfr

import (
	"encoding"
	"io"
)

// NodeType is the type of node in an extensive-form game tree.
type NodeType int

const (
	ChanceNodeType NodeType = iota
	TerminalNodeType
	PlayerNodeType
)

// InfoSet is the observable game history from the point of view of one player.
type InfoSet interface {
	// Key is an identifier used to uniquely look up this InfoSet
	// when accumulating probabilities in tabular CFR.
	//
	// It may be an arbitrary string of bytes and does not need to be
	// human-readable. For example, it could be a simplified abstraction
	// or hash of the full game history.
	Key() string
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// ChanceNode is a node that has a pre-defined probability distribution over its children.
type ChanceNode interface {
	// Get the probability of the ith child of this node.
	// May only be called for nodes with Type == Chance.
	GetChildProbability(i int) float64

	// Sample a single child from this Chance node according to the probability
	// distribution over children.
	//
	// Implementations may reuse sampling.SampleChanceNode to sample from the CDF,
	// (by scanning over GetChildProbability) or implement their own more efficient
	// sampling.
	SampleChild() (child GameTreeNode, p float64)
}

// PlayerNode is a node in which one of the player's acts.
type PlayerNode interface {
	// Player returns this current node's acting player.
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for this node for the given player.
	InfoSet(player int) InfoSet
	// InfoSetKey returns the equivalent of InfoSet(player).Key(),
	// but can be used to avoid allocations incurred by the InfoSet interface.
	InfoSetKey(player int) string
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float64
}

// Tree node represents a node in a directed rooted tree.
type TreeNode interface {
	// The number of direct children of this node.
	NumChildren() int
	// Get the ith child of this node.
	GetChild(i int) GameTreeNode
	// Get the parent of this node.
	Parent() GameTreeNode
}

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NodeType returns the type of game node.
	Type() NodeType
	// Release resources held by this node (including any children).
	Close()

	TreeNode
	ChanceNode
	PlayerNode
}

// StrategyProfile maintains a collection of regret-matching policies for each
// player node in the game tree.
//
// The policytable and deepcfr packages provide implementations of StrategyProfile.
type StrategyProfile interface {
	// GetPolicy returns the NodePolicy for the given node.
	GetPolicy(node GameTreeNode) NodePolicy

	// Calculate the next strategy profile for all visited nodes.
	Update()
	// Get the current iteration (number of times update has been called).
	Iter() int

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	io.Closer
}

// NodePolicy maintains the action policy for a single Player node.
type NodePolicy interface {
	// AddRegret provides new observed instantaneous regrets
	// to add to the total accumulated regret with the given weight.
	AddRegret(w float32, samplingQ, instantaneousRegrets []float32)
	// GetStrategy gets the current vector of probabilities with which the ith
	// available action should be played.
	GetStrategy() []float32

	// GetBaseline gets the current vector of action-dependend baseline values,
	// used in VR-MCCFR.
	GetBaseline() []float32
	// UpdateBaseline updates the current vector of baseline values.
	UpdateBaseline(w float32, action int, value float32)

	// AddStrategyWeight adds the current strategy with weight w to the average.
	AddStrategyWeight(w float32)
	// GetAverageStrategy returns the average strategy over all iterations.
	GetAverageStrategy() []float32

	// IsEmpty returns true if the NodePolicy is new and has no accumulated regret.
	IsEmpty() bool
}
