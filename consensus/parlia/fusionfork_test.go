package parlia

import (
	"container/heap"
	"math/big"
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
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
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
				common.HexToAddress("0x1"),
				common.HexToAddress("0x2"),
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
		for i := 0; i < len(tc.validators); i++ {
			if tc.votingPowers[i] > 0 {
				h = append(h, ValidatorItem{
					address:     tc.validators[i],
					votingPower: new(big.Int).Mul(big.NewInt(int64(tc.votingPowers[i])), big.NewInt(1e10)),
				})
			}
		}
		hp := &h
		heap.Init(hp)

		length := tc.k
		if length > len(h) {
			length = len(h)
		}
		eligibleValidators := make([]common.Address, length)
		eligibleVotingPowers := make([]uint64, length)
		for i := 0; i < length; i++ {
			item := heap.Pop(hp).(ValidatorItem)
			eligibleValidators[i] = item.address
			eligibleVotingPowers[i] = new(big.Int).Div(item.votingPower, big.NewInt(1e10)).Uint64() // to avoid overflow
		}

		// check
		if len(eligibleValidators) != len(tc.expected) {
			t.Errorf("expected %d, got %d", len(tc.expected), len(h))
		}
		for i := 0; i < len(tc.expected); i++ {
			if eligibleValidators[i] != tc.expected[i] {
				t.Errorf("expected %s, got %s", tc.expected[i].Hex(), h[i].address.Hex())
			}
		}
	}
}
