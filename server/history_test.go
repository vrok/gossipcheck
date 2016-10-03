package server

import "testing"

func TestHistory(t *testing.T) {
	type op struct {
		value    string
		expected bool
	}

	cases := []struct {
		size int
		ops  []op
	}{
		{
			1,
			[]op{
				op{"a", false},
				op{"a", true},
				op{"a", true},
				op{"b", false}, // Evict a
				op{"a", false},
			},
		},
		{
			2,
			[]op{
				op{"a", false},
				op{"b", false},
				op{"a", true},
				op{"c", false}, // Evict a
				op{"b", true},
				op{"a", false}, // Evict b
				op{"b", false}, // Evict c
				op{"c", false},
			},
		},
		{
			100,
			[]op{
				op{"a", false},
				op{"b", false},
				op{"c", false},
				op{"d", false},
				op{"e", false},
				op{"a", true},
				op{"b", true},
				op{"c", true},
				op{"d", true},
				op{"e", true},
			},
		},
	}

	for _, c := range cases {
		h := NewHistory(c.size, c.size)
		for _, op := range c.ops {
			if h.Observe(&Message{ID: op.value}) != op.expected {
				t.Errorf("Wrong result, expected %t, got %t", op.expected, !op.expected)
			}
		}
	}
}
