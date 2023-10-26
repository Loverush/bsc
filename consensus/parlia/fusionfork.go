package parlia

import (
	"container/heap"
	"context"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/systemcontracts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// init contract
func (p *Parlia) initFusionContract(state *state.StateDB, header *types.Header, chain core.ChainContext,
	txs *[]*types.Transaction, receipts *[]*types.Receipt, receivedTxs *[]*types.Transaction, usedGas *uint64, mining bool,
) error {
	// method
	method := "initialize"
	// contracts
	contracts := []string{
		systemcontracts.StakeHubContract,
	}
	// get packed data
	data, err := p.stakeHubABI.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for init fusion contract", "error", err)
		return err
	}
	for _, c := range contracts {
		msg := p.getSystemMessage(header.Coinbase, common.HexToAddress(c), data, common.Big0)
		// apply message
		log.Trace("init fusion contract", "block hash", header.Hash(), "contract", c)
		err = p.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
		if err != nil {
			return err
		}
	}
	return nil
}

type ValidatorItem struct {
	address     common.Address
	votingPower *big.Int
}

// An ValidatorHeap is a min-heap of validator's votingPower.
type ValidatorHeap []ValidatorItem

func (h ValidatorHeap) Len() int { return len(h) }

// We want topK validators with max voting power, so we need a min-heap
func (h ValidatorHeap) Less(i, j int) bool {
	if h[i].votingPower.Cmp(h[j].votingPower) == 0 {
		return h[i].address.Hex() < h[j].address.Hex()
	} else {
		return h[i].votingPower.Cmp(h[j].votingPower) == -1
	}
}
func (h ValidatorHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *ValidatorHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(ValidatorItem))
}

func (h *ValidatorHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (p *Parlia) updateEligibleValidators(state *state.StateDB, header *types.Header, chain core.ChainContext,
	txs *[]*types.Transaction, receipts *[]*types.Receipt, receivedTxs *[]*types.Transaction, usedGas *uint64, mining bool,
) error {
	// 1. get all validators and its voting power
	method := "getValidatorWithVotingPower"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	blockNr := rpc.BlockNumberOrHashWithHash(header.ParentHash, false)
	toAddress := common.HexToAddress(systemcontracts.StakeHubContract)
	gas := (hexutil.Uint64)(uint64(math.MaxUint64 / 2))

	var allValidators []common.Address
	var allVotingPowers []*big.Int
	for {
		data, err := p.stakeHubABI.Pack(method, big.NewInt(int64(len(allValidators))), big.NewInt(100))
		if err != nil {
			log.Error("Unable to pack tx for getValidatorWithVotingPower", "error", err)
			return err
		}
		msgData := (hexutil.Bytes)(data)

		result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
			Gas:  &gas,
			To:   &toAddress,
			Data: &msgData,
		}, blockNr, nil, nil)
		if err != nil {
			return err
		}

		var validators []common.Address
		var votingPowers []*big.Int
		var totalLength *big.Int
		if err := p.stakeHubABI.UnpackIntoInterface(&[]interface{}{&validators, &votingPowers, &totalLength}, method, result); err != nil {
			return err
		}

		allValidators = append(allValidators, validators...)
		allVotingPowers = append(allVotingPowers, votingPowers...)
		if totalLength.Cmp(big.NewInt(int64(len(allValidators)))) == 0 {
			break
		}
	}

	// 2. sort by voting power
	method = "maxElectedValidators"
	data, err := p.stakeHubABI.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for maxElectedValidators", "error", err)
		return err
	}
	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		Gas:  &gas,
		To:   &toAddress,
		Data: &msgData,
	}, blockNr, nil, nil)
	if err != nil {
		return err
	}

	var maxElectedValidators *big.Int
	if err := p.stakeHubABI.UnpackIntoInterface(&maxElectedValidators, method, result); err != nil {
		return err
	}

	var validatorHeap ValidatorHeap
	if maxElectedValidators.Cmp(big.NewInt(int64(len(allValidators)))) == 1 {
		for i := 0; i < len(allValidators); i++ {
			if allVotingPowers[i].Cmp(big.NewInt(0)) == 1 {
				validatorHeap = append(validatorHeap, ValidatorItem{
					address:     allValidators[i],
					votingPower: allVotingPowers[i],
				})
			}
		}
		sort.SliceStable(validatorHeap, validatorHeap.Less)
	} else {
		i := 0
		for len(validatorHeap) < int(maxElectedValidators.Int64()) && i < len(allValidators) {
			if allVotingPowers[i].Cmp(big.NewInt(0)) == 1 {
				validatorHeap = append(validatorHeap, ValidatorItem{
					address:     allValidators[i],
					votingPower: allVotingPowers[i],
				})
			}
			i++
		}
		hp := &validatorHeap
		heap.Init(hp)
		for j := i; j < len(allValidators); j++ {
			if allVotingPowers[j].Cmp(validatorHeap[0].votingPower) == 1 {
				heap.Pop(hp)
				heap.Push(hp, ValidatorItem{
					address:     allValidators[j],
					votingPower: allVotingPowers[j],
				})
			}
		}
	}

	length := len(validatorHeap)
	eligibleValidators := make([]common.Address, length)
	eligibleVotingPowers := make([]uint64, length)
	// reverse(from max to min)
	for i, item := range validatorHeap {
		eligibleValidators[length-i-1] = item.address
		eligibleVotingPowers[length-i-1] = new(big.Int).Div(item.votingPower, big.NewInt(1e10)).Uint64() // to avoid overflow
	}

	// 3. update validator set to system contract
	method = "updateEligibleValidators"
	data, err = p.stakeHubABI.Pack(method, eligibleValidators, eligibleVotingPowers)
	if err != nil {
		log.Error("Unable to pack tx for updateEligibleValidators", "error", err)
		return err
	}

	// get system message
	msg := p.getSystemMessage(header.Coinbase, common.HexToAddress(systemcontracts.StakeHubContract), data, common.Big0)
	// apply message
	return p.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
}

func (p *Parlia) updateValidatorSetV2(state *state.StateDB, header *types.Header, chain core.ChainContext,
	txs *[]*types.Transaction, receipts *[]*types.Receipt, receivedTxs *[]*types.Transaction, usedGas *uint64, mining bool,
) error {
	// method
	method := "updateValidatorSetV2"

	// get packed data
	data, err := p.validatorSetABI.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for updateValidatorSetV2", "error", err)
		return err
	}

	// get system message
	msg := p.getSystemMessage(header.Coinbase, common.HexToAddress(systemcontracts.ValidatorContract), data, big.NewInt(0))
	// apply message
	return p.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
}
