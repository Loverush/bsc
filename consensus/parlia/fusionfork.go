package parlia

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"math/big"

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

// initializeFusionContract initialize new contracts of fusion fork
func (p *Parlia) initializeFusionContract(state *state.StateDB, header *types.Header, chain core.ChainContext,
	txs *[]*types.Transaction, receipts *[]*types.Receipt, receivedTxs *[]*types.Transaction, usedGas *uint64, mining bool,
) error {
	// method
	method := "initialize"
	// contracts
	// TODO: add all contracts that need to be initialized
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
		log.Info("initialize fusion contract", "block hash", header.Hash(), "contract", c)
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

// An ValidatorHeap is a max-heap of validator's votingPower.
type ValidatorHeap []ValidatorItem

func (h *ValidatorHeap) Len() int { return len(*h) }

func (h *ValidatorHeap) Less(i, j int) bool {
	// We want topK validators with max voting power, so we need a max-heap
	if (*h)[i].votingPower.Cmp((*h)[j].votingPower) == 0 {
		return (*h)[i].address.Hex() < (*h)[j].address.Hex()
	} else {
		return (*h)[i].votingPower.Cmp((*h)[j].votingPower) == 1
	}
}

func (h *ValidatorHeap) Swap(i, j int) { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }

func (h *ValidatorHeap) Push(x interface{}) {
	*h = append(*h, x.(ValidatorItem))
}

func (h *ValidatorHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (p *Parlia) updateEligibleValidatorSet(state *state.StateDB, header *types.Header, chain core.ChainContext,
	txs *[]*types.Transaction, receipts *[]*types.Receipt, receivedTxs *[]*types.Transaction, usedGas *uint64, mining bool,
) error {
	// 1. get all validators and its voting power
	blockNr := rpc.BlockNumberOrHashWithHash(header.ParentHash, false)
	validators, votingPowers, voteAddrs, err := p.getValidatorElectionInfo(blockNr)
	if err != nil {
		return err
	}
	maxElectedValidators, err := p.getMaxElectedValidators(blockNr)
	if err != nil {
		return err
	}

	// 2. sort by voting power
	eValidators, eVotingPowers, eVoteAddrs := getTopValidatorsByVotingPower(validators, votingPowers, voteAddrs, maxElectedValidators)

	// 3. update validator set to system contract
	method := "updateEligibleValidatorSet"
	data, err := p.stakeHubABI.Pack(method, eValidators, eVotingPowers, eVoteAddrs)
	if err != nil {
		log.Error("Unable to pack tx for updateEligibleValidatorSet", "error", err)
		return err
	}

	// get system message
	msg := p.getSystemMessage(header.Coinbase, common.HexToAddress(systemcontracts.StakeHubContract), data, common.Big0)
	// apply message
	return p.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
}

func (p *Parlia) getValidatorElectionInfo(blockNr rpc.BlockNumberOrHash) (validators []common.Address, votingPowers []*big.Int, voteAddrs [][]byte, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := "getValidatorWithVotingPower"
	toAddress := common.HexToAddress(systemcontracts.StakeHubContract)
	gas := (hexutil.Uint64)(uint64(math.MaxUint64 / 2))

	data, err := p.stakeHubABI.Pack(method, big.NewInt(0), big.NewInt(0))
	if err != nil {
		log.Error("Unable to pack tx for getValidatorWithVotingPower", "error", err)
		return nil, nil, nil, err
	}
	msgData := (hexutil.Bytes)(data)

	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		Gas:  &gas,
		To:   &toAddress,
		Data: &msgData,
	}, blockNr, nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	var totalLength *big.Int
	if err := p.stakeHubABI.UnpackIntoInterface(&[]interface{}{&validators, &votingPowers, &voteAddrs, &totalLength}, method, result); err != nil {
		return nil, nil, nil, err
	}
	if totalLength.Int64() != int64(len(validators)) {
		return nil, nil, nil, fmt.Errorf("validator length not match")
	}

	return validators, votingPowers, voteAddrs, nil
}

func (p *Parlia) getMaxElectedValidators(blockNr rpc.BlockNumberOrHash) (maxElectedValidators *big.Int, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := "maxElectedValidators"
	toAddress := common.HexToAddress(systemcontracts.StakeHubContract)
	gas := (hexutil.Uint64)(uint64(math.MaxUint64 / 2))

	data, err := p.stakeHubABI.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for maxElectedValidators", "error", err)
		return nil, err
	}
	msgData := (hexutil.Bytes)(data)

	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		Gas:  &gas,
		To:   &toAddress,
		Data: &msgData,
	}, blockNr, nil, nil)
	if err != nil {
		return nil, err
	}

	if err := p.stakeHubABI.UnpackIntoInterface(&maxElectedValidators, method, result); err != nil {
		return nil, err
	}

	return maxElectedValidators, nil
}

func getTopValidatorsByVotingPower(validators []common.Address, votingPowers []*big.Int, voteAddrs [][]byte, maxElectedValidators *big.Int) ([]common.Address, []uint64, [][]byte) {
	var validatorHeap ValidatorHeap
	for i := 0; i < len(validators); i++ {
		// only keep validators with voting power > 0
		if votingPowers[i].Cmp(big.NewInt(0)) == 1 {
			validatorHeap = append(validatorHeap, ValidatorItem{
				address:     validators[i],
				votingPower: votingPowers[i],
			})
		}
	}
	hp := &validatorHeap
	heap.Init(hp)

	length := int(maxElectedValidators.Int64())
	if length > len(validatorHeap) {
		length = len(validatorHeap)
	}
	eValidators := make([]common.Address, length)
	eVotingPowers := make([]uint64, length)
	eVoteAddrs := make([][]byte, length)
	for i := 0; i < length; i++ {
		item := heap.Pop(hp).(ValidatorItem)
		eValidators[i] = item.address
		eVotingPowers[i] = new(big.Int).Div(item.votingPower, big.NewInt(1e10)).Uint64() // to avoid overflow
		eVoteAddrs[i] = voteAddrs[i]
	}

	return eValidators, eVotingPowers, eVoteAddrs
}
