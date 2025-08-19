// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package processor

import (
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// assertSortOrder verifies that the actual slice matches the expected order
func assertSortOrder(t *testing.T, actual, expected []schema.GroupVersion) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("Expected %d group versions, got %d", len(expected), len(actual))
	}

	for i, gv := range actual {
		if gv != expected[i] {
			t.Errorf("Position %d: expected %v, got %v", i, expected[i], gv)
		}
	}
}

func TestCompareGroupsHierarchically(t *testing.T) {
	tests := []struct {
		name     string
		group1   string
		group2   string
		expected int
	}{
		// Hierarchical domain relationships
		{"parent before subdomain", "k8s.io", "apps.k8s.io", -1},
		{"subdomain after parent", "apps.k8s.io", "k8s.io", 1},
		{"nested hierarchy", "networking.k8s.io", "gateway.networking.k8s.io", -1},

		// Alphabetical sorting of siblings
		{"siblings alphabetical", "apps.k8s.io", "batch.k8s.io", -1},
		{"siblings reverse", "batch.k8s.io", "apps.k8s.io", 1},
		{"different domains", "example.com", "test.org", -1},

		// Edge cases
		{"equal groups", "apps.k8s.io", "apps.k8s.io", 0},
		{"core vs named", "", "apps.k8s.io", -1},
		{"named vs core", "apps.k8s.io", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareGroupsHierarchically(tt.group1, tt.group2, []string{})
			if result != tt.expected {
				t.Errorf("compareGroupsHierarchically(%q, %q) = %d, want %d", tt.group1, tt.group2, result, tt.expected)
			}
		})
	}
}

func TestHierarchicalSortingIntegration(t *testing.T) {
	// Test the complete sorting behavior with a realistic set of API groups
	groupVersions := []schema.GroupVersion{
		{Group: "storage.k8s.io", Version: "v1"},
		{Group: "apps.k8s.io", Version: "v1"},
		{Group: "k8s.io", Version: "v1"},
		{Group: "batch.k8s.io", Version: "v1"},
		{Group: "networking.k8s.io", Version: "v1"},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1"},
		{Group: "events.k8s.io", Version: "v1"},
		{Group: "", Version: "v1"}, // core group
		{Group: "apps.k8s.io", Version: "v1beta1"},
		{Group: "gateway.networking.k8s.io", Version: "v1alpha2"},
		{Group: "x-k8s.io", Version: "v1"},                           // x-k8s.io domain
		{Group: "metrics.x-k8s.io", Version: "v1beta1"},              // x-k8s.io subdomain
		{Group: "example.com", Version: "v1"},                        // other domain
		{Group: "test.example.com", Version: "v1"},                   // other subdomain
	}

	// Sort using hierarchical comparison with no special patterns
	sort.SliceStable(groupVersions, compareGroupVersionsFunction([]string{}))

	expected := []schema.GroupVersion{
		{Group: "", Version: "v1"},                                    // core group (empty string sorts first alphabetically)
		{Group: "example.com", Version: "v1"},                        // example.com parent domain
		{Group: "test.example.com", Version: "v1"},                   // example.com subdomain
		{Group: "k8s.io", Version: "v1"},                             // k8s.io parent domain
		{Group: "apps.k8s.io", Version: "v1"},                        // k8s.io subdomain, v1 before v1beta1
		{Group: "apps.k8s.io", Version: "v1beta1"},                   // k8s.io subdomain, v1beta1 after v1
		{Group: "batch.k8s.io", Version: "v1"},                       // k8s.io sibling to apps
		{Group: "events.k8s.io", Version: "v1"},                      // k8s.io sibling to apps and batch
		{Group: "networking.k8s.io", Version: "v1"},                  // k8s.io sibling to apps, batch, events
		{Group: "gateway.networking.k8s.io", Version: "v1alpha2"},    // k8s.io nested subdomain, v1alpha2 before v1beta1
		{Group: "gateway.networking.k8s.io", Version: "v1beta1"},     // k8s.io nested subdomain, v1beta1 after v1alpha2
		{Group: "storage.k8s.io", Version: "v1"},                     // k8s.io sibling to networking
		{Group: "x-k8s.io", Version: "v1"},                           // x-k8s.io parent domain
		{Group: "metrics.x-k8s.io", Version: "v1beta1"},              // x-k8s.io subdomain
	}

	assertSortOrder(t, groupVersions, expected)
}

func TestCustomGroupSort(t *testing.T) {
	// Test with custom group sort configuration
	customSortPatterns := []string{
		"custom.example.com", // Priority 0 (highest)
		"",                   // Priority 1 (core APIs)
		"k8s.io",            // Priority 2 (k8s.io and subdomains)
		// Note: No explicit "other" pattern needed - unmatched groups are automatically handled
	}

	groupVersions := []schema.GroupVersion{
		{Group: "apps.k8s.io", Version: "v1"},
		{Group: "", Version: "v1"},
		{Group: "custom.example.com", Version: "v1"},
		{Group: "api.custom.example.com", Version: "v1"},
		{Group: "other.example.com", Version: "v1"},
	}

	// Sort using custom patterns
	sort.SliceStable(groupVersions, compareGroupVersionsFunction(customSortPatterns))

	expected := []schema.GroupVersion{
		{Group: "custom.example.com", Version: "v1"},     // custom priority 0
		{Group: "api.custom.example.com", Version: "v1"}, // custom priority 0
		{Group: "", Version: "v1"},                       // core priority 1
		{Group: "apps.k8s.io", Version: "v1"},           // k8s priority 2
		{Group: "other.example.com", Version: "v1"},     // implicit "other" priority 3
	}

	assertSortOrder(t, groupVersions, expected)
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		patterns []string
		expected int
	}{
		// Exact matches and subdomain matching
		{"exact match", "k8s.io", []string{"k8s.io"}, 0},
		{"subdomain match", "apps.k8s.io", []string{"k8s.io"}, 0},
		{"nested subdomain", "gateway.networking.k8s.io", []string{"k8s.io"}, 0},
		{"intermediate parent", "gateway.networking.k8s.io", []string{"networking.k8s.io"}, 0},

		// Non-matches (implicit "other" group)
		{"different domain", "example.com", []string{"k8s.io"}, 1},
		{"partial match", "notk8s.io", []string{"k8s.io"}, 1},
		{"parent vs subdomain", "k8s.io", []string{"apps.k8s.io"}, 1},

		// Special patterns
		{"wildcard", "anything.example.com", []string{"*"}, 0},
		{"empty exact", "", []string{""}, 0},
		{"empty no match", "apps.k8s.io", []string{""}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getGroupPriority(tt.group, tt.patterns)
			if result != tt.expected {
				t.Errorf("getGroupPriority(%q, %v) = %d, want %d", tt.group, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestMultiplePatternPriorities(t *testing.T) {
	// Test priority ordering with multiple patterns
	patterns := []string{"", "k8s.io", "example.com"}

	tests := []struct {
		group    string
		expected int
	}{
		{"", 0},                    // matches first pattern
		{"k8s.io", 1},             // matches second pattern
		{"apps.k8s.io", 1},        // subdomain of second pattern
		{"example.com", 2},        // matches third pattern
		{"test.example.com", 2},   // subdomain of third pattern
		{"other.org", 3},          // no match, implicit "other"
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			result := getGroupPriority(tt.group, patterns)
			if result != tt.expected {
				t.Errorf("getGroupPriority(%q, %v) = %d, want %d", tt.group, patterns, result, tt.expected)
			}
		})
	}
}
