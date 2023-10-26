package parlia

import (
	"container/heap"
	"math/big"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestValidatorHeap(t *testing.T) {
	testCases := []struct {
		description  string
		k            int
		validators   []common.Address
		votingPowers []uint64
		expected     []common.Address
	}{
		{
			description: "normal case",
			k:           2,
			validators: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
				common.HexToAddress("0x3"),
			},
			votingPowers: []uint64{
				300,
				200,
				100,
			},
			expected: []common.Address{
				common.HexToAddress("0x2"),
				common.HexToAddress("0x1"),
			},
		},
		{
			description: "same voting power",
			k:           2,
			validators: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
				common.HexToAddress("0x3"),
			},
			votingPowers: []uint64{
				300,
				100,
				100,
			},
			expected: []common.Address{
				common.HexToAddress("0x2"),
				common.HexToAddress("0x1"),
			},
		},
		{
			description: "zero voting power and k > len(validators)",
			k:           5,
			validators: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
				common.HexToAddress("0x3"),
				common.HexToAddress("0x4"),
			},
			votingPowers: []uint64{
				300,
				0,
				0,
				0,
			},
			expected: []common.Address{
				common.HexToAddress("0x1"),
			},
		},
		{
			description: "zero voting power and k < len(validators)",
			k:           2,
			validators: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
				common.HexToAddress("0x3"),
				common.HexToAddress("0x4"),
			},
			votingPowers: []uint64{
				300,
				0,
				0,
				0,
			},
			expected: []common.Address{
				common.HexToAddress("0x1"),
			},
		},
		{
			description: "all zero voting power",
			k:           2,
			validators: []common.Address{
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
				common.HexToAddress("0x3"),
				common.HexToAddress("0x4"),
			},
			votingPowers: []uint64{
				0,
				0,
				0,
				0,
			},
			expected: []common.Address{},
		},
	}
	for _, tc := range testCases {
		var h ValidatorHeap
		if tc.k > len(tc.validators) {
			for i := 0; i < len(tc.validators); i++ {
				if tc.votingPowers[i] > 0 {
					h = append(h, ValidatorItem{
						address:     tc.validators[i],
						votingPower: big.NewInt(int64(tc.votingPowers[i])),
					})
				}
			}
			sort.SliceStable(h, h.Less)
		} else {
			i := 0
			for len(h) < tc.k && i < len(tc.validators) {
				if tc.votingPowers[i] > 0 {
					h = append(h, ValidatorItem{
						address:     tc.validators[i],
						votingPower: big.NewInt(int64(tc.votingPowers[i])),
					})
				}
				i++
			}
			hp := &h
			heap.Init(hp)
			for j := i; j < len(tc.validators); j++ {
				if tc.votingPowers[i] > h[0].votingPower.Uint64() {
					heap.Pop(hp)
					heap.Push(hp, ValidatorItem{
						address:     tc.validators[j],
						votingPower: big.NewInt(int64(tc.votingPowers[i])),
					})
				}
			}
		}

		// check
		if len(h) != len(tc.expected) {
			t.Errorf("expected %d, got %d", len(tc.expected), len(h))
		}
		for i := 0; i < len(tc.expected); i++ {
			if h[i].address != tc.expected[i] {
				t.Errorf("expected %s, got %s", tc.expected[i].Hex(), h[i].address.Hex())
			}
		}
	}
}
