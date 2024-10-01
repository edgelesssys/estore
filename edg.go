/*
Copyright (c) Edgeless Systems GmbH

SPDX-License-Identifier: AGPL-3.0-only
*/

package estore

import (
	"encoding/binary"

	"github.com/cockroachdb/errors"
)

var edgMonotonicCounterKey = []byte("!EDGELESS_MONOTONIC_COUNTER")

func (d *DB) edgGetMonotonicCounterFromStore() (uint64, error) {
	value, closer, err := d.Get(edgMonotonicCounterKey)
	if errors.Is(err, ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	return binary.LittleEndian.Uint64(value), nil
}

func (d *DB) edgSetMonotonicCounterOnStore(value uint64) error {
	return d.Set(edgMonotonicCounterKey, binary.LittleEndian.AppendUint64(nil, value), nil)
}

func (d *DB) edgVerifyFreshness() error {
	if d.opts.SetMonotonicCounter == nil {
		return nil
	}

	// get counter from trusted source
	sourceCount, err := d.opts.SetMonotonicCounter(0)
	if err != nil {
		return errors.Wrap(err, "getting monotonic counter from trusted source")
	}

	// get counter from store
	storeCount, err := d.edgGetMonotonicCounterFromStore()
	if err != nil {
		return errors.Wrap(err, "getting monotonic counter from store")
	}

	if storeCount < sourceCount {
		return errors.Newf("rollback detected: store counter: %v, trusted source counter: %v", storeCount, sourceCount)
	}
	if storeCount > sourceCount {
		d.opts.Logger.Infof("WARNING: open: monotonic counter source lags behind: store counter: %v, source counter: %v", storeCount, sourceCount)
		// will be synced on next tx commit
	}

	d.monotonicCounter = storeCount
	return nil
}
