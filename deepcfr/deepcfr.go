package deepcfr

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

// DeepCFR implements cfr.StrategyProfile, and uses function approximation
// to estimate strategies rather than accumulation of regrets for all
// infosets. This can be more tractable for large games where storing
// all of the regrets for all infosets is impractical.
//
// During CFR iterations, samples are added to the given buffer.
// When NextIter is called, the model is retrained.
type DeepCFR struct {
	model         Model
	buffers       []Buffer
	trainedModels [][]TrainedModel
	iter          int
}

// New returns a new DeepCFR policy with the given model and sample buffer.
func New(model Model, buffers []Buffer) *DeepCFR {
	return &DeepCFR{
		model:   model,
		buffers: buffers,
		trainedModels: [][]TrainedModel{
			[]TrainedModel{},
			[]TrainedModel{},
		},
		iter: 1,
	}
}

func (d *DeepCFR) currentPlayer() int {
	return d.iter % 2
}

func (d *DeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &dcfrPolicy{
		node:   node,
		buf:    d.buffers[node.Player()],
		models: d.trainedModels[node.Player()],
		iter:   d.iter,
	}
}

// Update implements cfr.StrategyProfile.
func (d *DeepCFR) Update() {
	player := d.currentPlayer()
	buf := d.buffers[player]
	trained := d.model.Train(buf)
	d.trainedModels[player] = append(d.trainedModels[player], trained)

	d.iter++
}

// Iter implements cfr.StrategyProfile.
func (d *DeepCFR) Iter() int {
	return d.iter
}

func (d *DeepCFR) GetBuffer(player int) Buffer {
	return d.buffers[player]
}

func (d *DeepCFR) SampleModel() TrajectorySampledSDCFR {
	models := sampleModels(d.trainedModels)
	return TrajectorySampledSDCFR(models)
}

func (d *DeepCFR) Close() error {
	for _, buf := range d.buffers {
		if err := buf.Close(); err != nil {
			return err
		}
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
// Note that to be able to use this method, the concrete types
// implementing the Model, TrainedModel, and Buffers must be registered
// with gob.
func (d *DeepCFR) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Need to pass pointer to interface so that Gob sees the interface rather
	// than the concrete type. See the example in encoding/gob.
	if err := enc.Encode(&d.model); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.buffers); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.trainedModels); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.iter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (d *DeepCFR) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&d.model); err != nil {
		return err
	}

	if err := dec.Decode(&d.buffers); err != nil {
		return err
	}

	if err := dec.Decode(&d.trainedModels); err != nil {
		return err
	}

	if err := dec.Decode(&d.iter); err != nil {
		return err
	}

	return nil
}

type dcfrPolicy struct {
	node            cfr.GameTreeNode
	buf             Buffer
	models          []TrainedModel
	currentStrategy []float32
	iter            int
}

func (d *dcfrPolicy) currentModel() TrainedModel {
	if len(d.models) == 0 {
		return nil
	}

	return d.models[len(d.models)-1]
}

func (d *dcfrPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	w *= float32((d.iter + 1) / 2) // Linear CFR.
	d.buf.AddSample(d.node, instantaneousRegrets, w)
}

func (d *dcfrPolicy) GetStrategy() []float32 {
	if d.currentStrategy == nil {
		model := d.currentModel()
		nChildren := d.node.NumChildren()
		if model == nil {
			d.currentStrategy = uniformDist(nChildren)
		} else {
			infoSet := d.node.InfoSet(d.node.Player())
			d.currentStrategy = regretMatching(model.Predict(infoSet, nChildren))
		}
	}

	return d.currentStrategy
}

func (d *dcfrPolicy) AddStrategyWeight(w float32) {
}

func (d *dcfrPolicy) GetAverageStrategy() []float32 {
	nChildren := d.node.NumChildren()
	if nChildren == 1 {
		return []float32{1.0}
	}

	var modelPredictions [][]float32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		modelPredictions = d.getModelPredictions()
		wg.Done()
	}()

	var modelWeights []float32
	wg.Add(1)
	go func() {
		modelWeights = d.getModelWeights()
		wg.Done()
	}()

	wg.Wait()

	result := make([]float32, nChildren)
	for t, w := range modelWeights {
		f32.AxpyUnitary(w, modelPredictions[t], result)
	}

	return result
}

func (d *dcfrPolicy) getModelPredictions() [][]float32 {
	var mu sync.Mutex
	var wg sync.WaitGroup
	modelPredictions := make([][]float32, len(d.models))
	infoSet := d.node.InfoSet(d.node.Player())
	nChildren := d.node.NumChildren()
	for i, model := range d.models {
		wg.Add(1)
		go func(i int, model TrainedModel) {
			result := regretMatching(model.Predict(infoSet, nChildren))
			mu.Lock()
			modelPredictions[i] = result
			mu.Unlock()
			wg.Done()
		}(i, model)
	}

	wg.Wait()
	return modelPredictions
}

func (d *dcfrPolicy) getModelWeights() []float32 {
	var mu sync.Mutex
	var wg sync.WaitGroup
	modelWeights := make([]float32, len(d.models))
	for i := range modelWeights {
		modelWeights[i] = 1.0
	}

	lastChild := d.node
	for node := d.node.Parent(); node != nil; node = node.Parent() {
		// Figure out which child of parent.
		childIdx := -1
		nChildren := node.NumChildren()
		for i := 0; i < nChildren; i++ {
			if node.GetChild(i) == lastChild {
				childIdx = i
				break
			}
		}

		if childIdx == -1 {
			panic(fmt.Errorf(
				"failed to identify action leading to history! node: %v, lastChild: %v",
				node, lastChild))
		}

		if node.Player() == d.node.Player() {
			infoSet := node.InfoSet(node.Player())
			for i, model := range d.models {
				wg.Add(1)
				go func(i int, model TrainedModel) {
					strategy := regretMatching(model.Predict(infoSet, nChildren))
					mu.Lock()
					modelWeights[i] *= strategy[childIdx]
					mu.Unlock()
					wg.Done()
				}(i, model)
			}
		}

		lastChild = node
	}

	wg.Wait()

	var normalization float32
	for t, w := range modelWeights {
		normalization += float32(t+1) * w
	}
	f32.ScalUnitary(1.0/normalization, modelWeights)
	return modelWeights
}

func regretMatching(advantages []float32) []float32 {
	makePositive(advantages)
	if total := f32.Sum(advantages); total > 0 {
		f32.ScalUnitary(1.0/total, advantages)
	} else { // Uniform probability.
		pUniform := 1.0 / float32(len(advantages))
		for i := range advantages {
			advantages[i] = pUniform
		}
	}

	return advantages
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

func init() {
	gob.Register(&DeepCFR{})
}
