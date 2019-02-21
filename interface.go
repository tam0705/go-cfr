package cfr

import "fmt"

type NodeType int

const (
	ChanceNode NodeType = iota
	TerminalNode
	PlayerNode
)

// InfoSet represents the information partition for a particular
// player at a particular point in the game.
type InfoSet struct {
	// Public is the component of the information set that is public
	// information available to all players.
	Public string
	// Private is the component of the information set that is hidden
	// knowledge known only by this player.
	Private string
}

// String implements fmt.Stringer.
func (is InfoSet) String() string {
	return fmt.Sprintf("{public: %s, private: %s}", is.Public, is.Private)
}

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NodeType returns the type of game node.
	Type() NodeType

	// Prepare children of this node.
	// Must be called before using NumChildren, GetChild, or GetChildProbability.
	// Implementations are free to ignore the request and generate children
	// on the fly as needed if they choose.
	BuildChildren()
	// Release resources allocated by calling BuildChildren.
	// After calling FreeChildren, NumChildren, GetChild, and GetChildProbability
	// may no longer be called (unless the node is rebuilt).
	FreeChildren()
	// The number of direct children of this node.
	NumChildren() int
	// Get the ith child of this node.
	GetChild(i int) GameTreeNode
	// Get the probability of the ith child of this node.
	// May only be called for nodes with Type == Chance.
	GetChildProbability(i int) float32
	// Sample one of the children of this node, according to the probability
	// distribution. Only applicable for nodes with Type == Chance.
	SampleChild() GameTreeNode

	// Player returns this current node's acting player.
	//
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for this node for the given player.
	InfoSet(player int) InfoSet
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float32
}
