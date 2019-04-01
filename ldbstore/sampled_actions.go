package ldbstore

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/sampling"
)

// LDBSampledActions implements cfr.SampledActions by storing
// all sampled actions in a LevelDB database on disk.
//
// It is functionally equivalent to cfr.SampledActionMap. In practice, it will be
// much slower but use a constant amount of memory even if the game tree is very large.
type LDBSampledActions struct {
	path  string
	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

// NewLDBSampledActions creates a new LDBSampledActions backed by
// the given LevelDB database.
func NewLDBSampledActions(path string, opts *opt.Options) (*LDBSampledActions, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}

	return &LDBSampledActions{path: path, db: db}, nil
}

// Close implements io.Closer.
func (l *LDBSampledActions) Close() error {
	if err := l.db.Close(); err != nil {
		glog.Errorf("error closing sampled action store: %v", err)
		return err
	}

	if err := os.RemoveAll(l.path); err != nil {
		glog.Errorf("error closing sampled action store: %v", err)
		return err
	}

	return nil
}

// Get implements cfr.SampledActions.
func (l *LDBSampledActions) Get(node cfr.GameTreeNode, policy cfr.NodePolicy) int {
	key := []byte(node.InfoSet(node.Player()).Key())
	buf, err := l.db.Get(key, l.rOpts)
	if err != nil {
		if err == leveldb.ErrNotFound {
			i := sampling.SampleOne(policy.GetStrategy())
			l.put(key, i)
			return i
		}

		panic(err)
	}

	i, ok := binary.Uvarint(buf)
	if ok <= 0 {
		panic(fmt.Errorf("error decoding buffer (%d): %v", ok, buf))
	}

	return int(i)
}

func (l *LDBSampledActions) put(key []byte, selected int) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(selected))
	if err := l.db.Put(key, buf[:n], l.wOpts); err != nil {
		panic(err)
	}
}