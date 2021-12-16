package sampling

import "github.com/tam0705/go-cfr"

// ExternalSampler implements cfr.Sampler by sampling all player actions.
type ExternalSampler struct {
	p []float32
}

func NewExternalSampler() *ExternalSampler {
	return &ExternalSampler{}
}

func (es *ExternalSampler) Sample(node cfr.GameTreeNode, policy cfr.NodePolicy) []float32 {
	nChildren := node.NumChildren()
	for len(es.p) < nChildren {
		es.p = append(es.p, 1.0)
	}

	return es.p[:nChildren]
}
