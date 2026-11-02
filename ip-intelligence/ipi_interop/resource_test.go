/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */
package ipi_interop

import "testing"

func TestNewResourceManager(t *testing.T) {
	t.Run("create new resource manager", func(t *testing.T) {
		manager := NewResourceManager()
		defer manager.Free()

		if manager == nil {
			t.Fatal("NewResourceManager returned nil")
		}

		// Check initial state
		if manager.CPtr == nil {
			t.Error("NewResourceManager created manager with nil CPtr")
		}

		if manager.setHeaders != nil {
			t.Error("NewResourceManager should initialize setHeaders as nil")
		}

		if manager.HttpHeaderKeys != nil {
			t.Error("NewResourceManager should initialize HttpHeaderKeys as nil")
		}
	})
}

func TestResourceManager_Free(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*ResourceManager)
		verify func(*testing.T, *ResourceManager)
	}{
		{
			name: "free empty manager",
			setup: func(m *ResourceManager) {
				// Default state, no setup needed
			},
			verify: func(t *testing.T, m *ResourceManager) {
				if m.CPtr != nil {
					t.Error("Free() did not set CPtr to nil")
				}
				if m.setHeaders != nil {
					t.Error("Free() did not set setHeaders to nil")
				}
				if m.HttpHeaderKeys != nil {
					t.Error("Free() did not set HttpHeaderKeys to nil")
				}
			},
		},
		{
			name: "free manager with initialized fields",
			setup: func(m *ResourceManager) {
				m.setHeaders = make(map[string][]string)
				m.HttpHeaderKeys = []EvidenceKey{{Prefix: EvidencePrefix(1), Key: "test"}}
			},
			verify: func(t *testing.T, m *ResourceManager) {
				if m.CPtr != nil {
					t.Error("Free() did not set CPtr to nil")
				}
				if m.setHeaders != nil {
					t.Error("Free() did not set setHeaders to nil")
				}
				if m.HttpHeaderKeys != nil {
					t.Error("Free() did not set HttpHeaderKeys to nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewResourceManager()
			if tt.setup != nil {
				tt.setup(manager)
			}

			manager.Free()

			if tt.verify != nil {
				tt.verify(t, manager)
			}
		})
	}
}

func TestResourceManager_FieldModification(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "modify setHeaders",
			test: func(t *testing.T) {
				manager := NewResourceManager()
				defer manager.Free()

				// Initialize and modify setHeaders
				manager.setHeaders = make(map[string][]string)
				manager.setHeaders["test"] = []string{"value1", "value2"}

				manager.Free()

				if manager.setHeaders != nil {
					t.Error("Free() did not clear setHeaders")
				}
			},
		},
		{
			name: "modify HttpHeaderKeys",
			test: func(t *testing.T) {
				manager := NewResourceManager()
				defer manager.Free()

				// Initialize and modify HttpHeaderKeys
				manager.HttpHeaderKeys = []EvidenceKey{
					{Prefix: EvidencePrefix(1), Key: "key1"},
					{Prefix: EvidencePrefix(2), Key: "key2"},
				}

				manager.Free()

				if manager.HttpHeaderKeys != nil {
					t.Error("Free() did not clear HttpHeaderKeys")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}
