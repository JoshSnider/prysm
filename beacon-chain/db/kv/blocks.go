package kv

import (
	"bytes"
	"context"

	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// Block retrieval by root.
func (k *Store) Block(ctx context.Context, blockRoot [32]byte) (*ethpb.BeaconBlock, error) {
	att := &ethpb.BeaconBlock{}
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		c := bkt.Cursor()
		for k, v := c.Seek(blockRoot[:]); k != nil && bytes.Contains(k, blockRoot[:]); k, v = c.Next() {
			if v != nil {
				return proto.Unmarshal(v, att)
			}
		}
		return nil
	})
	return att, err
}

// HeadBlock returns the latest canonical block in eth2.
func (k *Store) HeadBlock(ctx context.Context) (*ethpb.BeaconBlock, error) {
	headBlock := &ethpb.BeaconBlock{}
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(validatorsBucket)
		headRoot := bkt.Get(headBlockRootKey)
		if headRoot == nil {
			return nil
		}
		enc := bkt.Get(headRoot)
		if enc == nil {
			return nil
		}
		return proto.Unmarshal(enc, headBlock)
	})
	return headBlock, err
}

// Blocks retrieves a list of beacon blocks by filter criteria.
// TODO(#3164): Implement.
func (k *Store) Blocks(ctx context.Context, f *filters.QueryFilter) ([]*ethpb.BeaconBlock, error) {
	return nil, nil
}

// BlockRoots retrieves a list of beacon block roots by filter criteria.
// TODO(#3164): Implement.
func (k *Store) BlockRoots(ctx context.Context, f *filters.QueryFilter) ([][]byte, error) {
	return nil, nil
}

// HasBlock checks if a block by root exists in the db.
func (k *Store) HasBlock(ctx context.Context, blockRoot [32]byte) bool {
	exists := false
	// #nosec G104. Always returns nil.
	k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		c := bkt.Cursor()
		for k, v := c.Seek(blockRoot[:]); k != nil && bytes.Contains(k, blockRoot[:]); k, v = c.Next() {
			if v != nil {
				exists = true
				return nil
			}
		}
		return nil
	})
	return exists
}

// DeleteBlock by block root.
func (k *Store) DeleteBlock(ctx context.Context, blockRoot [32]byte) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		c := bkt.Cursor()
		for k, v := c.Seek(blockRoot[:]); k != nil && bytes.Contains(k, blockRoot[:]); k, v = c.Next() {
			if v != nil {
				return bkt.Delete(k)
			}
		}
		return nil
	})
}

// SaveBlock to the db.
func (k *Store) SaveBlock(ctx context.Context, block *ethpb.BeaconBlock) error {
	key, err := generateBlockKey(block)
	if err != nil {
		return err
	}
	enc, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(blocksBucket)
		return bucket.Put(key, enc)
	})
}

// SaveBlocks via batch updates to the db.
func (k *Store) SaveBlocks(ctx context.Context, blocks []*ethpb.BeaconBlock) error {
	encodedValues := make([][]byte, len(blocks))
	keys := make([][]byte, len(blocks))
	for i := 0; i < len(blocks); i++ {
		enc, err := proto.Marshal(blocks[i])
		if err != nil {
			return err
		}
		key, err := generateBlockKey(blocks[i])
		if err != nil {
			return err
		}
		encodedValues[i] = enc
		keys[i] = key
	}
	return k.db.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(blocksBucket)
		for i := 0; i < len(blocks); i++ {
			if err := bucket.Put(keys[i], encodedValues[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveHeadBlockRoot to the db.
func (k *Store) SaveHeadBlockRoot(ctx context.Context, blockRoot [32]byte) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(blocksBucket)
		return bucket.Put(headBlockRootKey, blockRoot[:])
	})
}

func generateBlockKey(block *ethpb.BeaconBlock) ([]byte, error) {
	buf := make([]byte, 0)
	buf = append(buf, []byte("parent-root")...)
	buf = append(buf, block.ParentRoot...)

	buf = append(buf, []byte("slot")...)
	buf = append(buf, uint64ToBytes(block.Slot)...)

	buf = append(buf, []byte("root")...)
	blockRoot, err := ssz.HashTreeRoot(block)
	if err != nil {
		return nil, err
	}
	buf = append(buf, blockRoot[:]...)
	return buf, nil
}