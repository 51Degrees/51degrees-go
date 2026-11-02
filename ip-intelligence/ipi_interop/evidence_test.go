package ipi_interop

import "testing"

func TestNewEvidence(t *testing.T) {
	t.Run("create new evidence", func(t *testing.T) {
		evidence := NewEvidence()

		if evidence == nil {
			t.Fatal("NewEvidence returned nil")
		}

		if evidence.cEvidence == nil {
			t.Error("NewEvidence created evidence with nil cEvidence")
		}

		if len(evidence.cEvidence) != 0 {
			t.Errorf("NewEvidence created evidence with non-empty cEvidence, got length %d",
				len(evidence.cEvidence))
		}

		if cap(evidence.cEvidence) != 0 {
			t.Errorf("NewEvidence created evidence with non-zero capacity, got %d",
				cap(evidence.cEvidence))
		}
	})
}
