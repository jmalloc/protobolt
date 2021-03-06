package protavobolt

import (
	bolt "github.com/coreos/bbolt"
	"github.com/jmalloc/protavo/src/protavo"
	"github.com/jmalloc/protavo/src/protavo/document"
	"github.com/jmalloc/protavo/src/protavobolt/internal/database"
)

// executeDelete deletes the given documents, provided their revisions match
// the currently persisted revisions.
func executeDelete(
	tx *bolt.Tx,
	ns string,
	doc *document.Document,
) error {
	s, ok, err := database.OpenStore(tx, ns)
	if err != nil {
		return err
	}

	var (
		rec    *database.Record
		exists bool
	)

	// load the record even if namespace store does not exist ...
	if ok {
		rec, exists, err = s.TryGetRecord(doc.ID)
		if err != nil {
			return err
		}
	}

	// ... but always check the revision
	if doc.Revision != rec.GetRevision() {
		return &protavo.OptimisticLockError{
			DocumentID: doc.ID,
			GivenRev:   doc.Revision,
			ActualRev:  rec.GetRevision(),
			Operation:  "delete",
		}
	}

	if !exists {
		return nil
	}

	if err := s.DeleteRecord(doc.ID); err != nil {
		return err
	}

	if err := s.DeleteContent(doc.ID); err != nil {
		return err
	}

	return s.UpdateKeys(doc.ID, rec.Keys, nil)
}
