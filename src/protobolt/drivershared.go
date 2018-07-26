package protobolt

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	bolt "github.com/coreos/bbolt"
)

// SharedDriver is an implementation of Driver that only opens the BoltDB database
// when performing an operation; thus allowing the database to be shared between
// multiple processes.
type SharedDriver struct {
	Path    string
	Mode    os.FileMode
	Options *bolt.Options

	m      sync.RWMutex
	closed bool
}

// View executes a read-only operation.
func (d *SharedDriver) View(ctx context.Context, op ViewOp) (bool, error) {
	d.m.RLock()
	defer d.m.RUnlock()

	db, ok, err := d.openR(ctx)
	if !ok || err != nil {
		return false, err
	}
	defer db.Close()

	return ok, db.View(func(tx *bolt.Tx) error {
		var err error
		ok, err = op.View(ctx, tx)
		return err
	})
}

// Update executes a read/write operation.
func (d *SharedDriver) Update(ctx context.Context, op UpdateOp) error {
	d.m.Lock()
	defer d.m.Unlock()

	db, err := d.openRW(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		return op.Update(ctx, tx)
	})
}

// openR opens the Bolt database in read-only mode.
func (d *SharedDriver) openR(ctx context.Context) (*bolt.DB, bool, error) {
	opts := d.Options

	if opts == nil {
		opts = &bolt.Options{}
		*opts = *bolt.DefaultOptions // copy the default options
	}

	opts.ReadOnly = true

	b, err := d.openDB(ctx, opts)

	// bbolt returns a "bad file descriptor" error when attempting to create a new
	// database file when the read-only option is specified.
	if _, ok := err.(*os.PathError); ok {
		return nil, false, nil
	}

	return b, true, err
}

// openRW opens the Bolt database in read/write mode.
func (d *SharedDriver) openRW(ctx context.Context) (*bolt.DB, error) {
	opts := d.Options

	if opts == nil {
		opts = &bolt.Options{}
		*opts = *bolt.DefaultOptions // copy the default options
	}

	return d.openDB(ctx, opts)
}

// openDB opens the BoltDB database. It uses a timeout derived from the ctx
// deadline.
func (d *SharedDriver) openDB(ctx context.Context, opts *bolt.Options) (*bolt.DB, error) {
	if d.closed {
		return nil, errors.New("database is closed")
	}

	if dl, ok := ctx.Deadline(); ok {
		opts.Timeout = time.Until(dl)
	}

	return bolt.Open(d.Path, d.Mode, opts)
}

// Close is a marks the database as closed, preventing future operations.
func (d *SharedDriver) Close() error {
	d.m.Lock()
	defer d.m.Unlock()

	d.closed = true

	return nil
}
