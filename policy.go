package cfr

import (
	"bytes"
	"encoding/gob"
	"expvar"
	"fmt"

	"github.com/tam0705/go-cfr/internal/policy"
)

var (
	numInfosets = expvar.NewInt("num_infosets")
)

func init() {
	gob.Register(&PolicyTable{})
}

// PolicyTable implements traditional (tabular) CFR by storing accumulated
// regrets and strategy sums for each InfoSet, which is looked up by its Key().
type PolicyTable struct {
	params DiscountParams
	iter   int

	// Map of InfoSet Key -> the policy for that infoset.
	PoliciesByKey map[string]*policy.Policy
	mayNeedUpdate map[*policy.Policy]struct{}
}

// NewPolicyTable creates a new PolicyTable with the given DiscountParams.
func NewPolicyTable(params DiscountParams) *PolicyTable {
	return &PolicyTable{
		params:        params,
		iter:          1,
		PoliciesByKey: make(map[string]*policy.Policy),
		mayNeedUpdate: make(map[*policy.Policy]struct{}),
	}
}

// Update performs regret matching for all nodes within this strategy profile that have
// been touched since the lapt call to Update().
func (pt *PolicyTable) Update() {
	discountPos, discountNeg, discountSum := pt.params.GetDiscountFactors(pt.iter)
	for p := range pt.mayNeedUpdate {
		p.NextStrategy(discountPos, discountNeg, discountSum)
		delete(pt.mayNeedUpdate, p)
	}

	pt.iter++
}

func (pt *PolicyTable) SetIter(val int) {
	pt.iter = val
} 

func (pt *PolicyTable) Iter() int {
	return pt.iter
}

func (pt *PolicyTable) Close() error {
	return nil
}

func (pt *PolicyTable) GetPolicyTable() map[string]*policy.Policy {
	return pt.PoliciesByKey
}

func (pt *PolicyTable) GetPolicy(node GameTreeNode) NodePolicy {
	key := string(node.InfoSetKey(node.Player()))
	np, ok := pt.PoliciesByKey[string(key)]
	if !ok {
		np = policy.New(node.NumChildren())
		pt.PoliciesByKey[string(key)] = np
		numInfosets.Set(int64(len(pt.PoliciesByKey)))
	} else if np.NumActions() != node.NumChildren() {
		panic(fmt.Errorf("strategy has n_actions=%v but node has n_children=%v: %v",
			np.NumActions(), node.NumChildren(), node))
	}

	pt.mayNeedUpdate[np] = struct{}{}
	return np
}

func (pt *PolicyTable) GetPolicyByKey(key string) (NodePolicy, bool) {
	np, ok := pt.PoliciesByKey[key]
	if !ok {
		np = policy.New(4)
		pt.PoliciesByKey[key] = np
		numInfosets.Set(int64(len(pt.PoliciesByKey)))
	}
	return np, true
}

func (pt *PolicyTable) SetStrategy(key string, strat []float32) {
	np, ok := pt.PoliciesByKey[key]
	if !ok {
		np = policy.New(len(strat))
		pt.PoliciesByKey[string(key)] = np
		numInfosets.Set(int64(len(pt.PoliciesByKey)))
	} else if np.NumActions() != len(strat) {
		panic(fmt.Errorf("strategy has n_actions=%v but strategy's size is=%v",
			np.NumActions(), len(strat)))
	}
	np.SetStrategy(strat)
	pt.PoliciesByKey[string(key)] = np
}

func (pt *PolicyTable) Iterate(iterator func(key string, strat []float32)) {
	for key, p := range pt.PoliciesByKey {
		iterator(key, p.GetStrategy())
	}
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (pt *PolicyTable) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&pt.params); err != nil {
		return err
	}

	if err := dec.Decode(&pt.iter); err != nil {
		return err
	}

	var nStrategies int
	if err := dec.Decode(&nStrategies); err != nil {
		return err
	}

	pt.PoliciesByKey = make(map[string]*policy.Policy, nStrategies)
	for i := 0; i < nStrategies; i++ {
		var key string
		if err := dec.Decode(&key); err != nil {
			return err
		}

		var p policy.Policy
		if err := dec.Decode(&p); err != nil {
			return err
		}

		pt.PoliciesByKey[key] = &p
	}

	pt.mayNeedUpdate = make(map[*policy.Policy]struct{})
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (pt *PolicyTable) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(pt.params); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.iter); err != nil {
		return nil, err
	}

	if err := enc.Encode(len(pt.PoliciesByKey)); err != nil {
		return nil, err
	}

	for key, p := range pt.PoliciesByKey {
		if err := enc.Encode(key); err != nil {
			return nil, err
		}

		if err := enc.Encode(p); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
