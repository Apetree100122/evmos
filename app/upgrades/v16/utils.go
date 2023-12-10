// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package v16

import (
	"fmt"
	"math/big"

	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v16/contracts"
	evmostypes "github.com/evmos/evmos/v16/types"
	"github.com/evmos/evmos/v16/utils"
	erc20keeper "github.com/evmos/evmos/v16/x/erc20/keeper"
	erc20types "github.com/evmos/evmos/v16/x/erc20/types"
	evmtypes "github.com/evmos/evmos/v16/x/evm/types"
)

const (
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
	WEVMOSContractTestnet = "0xcc491f589b45d4a3c679016195b3fb87d7848210"
)

// ConvertERC20Coins converts Native IBC coins from their ERC20 representation
// to the native representation. This also includes the withdrawal of WEVMOS tokens
// to EVMOS native tokens.
func ConvertERC20Coins(
	ctx sdk.Context,
	logger log.Logger,
	accountKeeper authkeeper.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
	erc20Keeper erc20keeper.Keeper,
) error {
	var wrappedContractAddr common.Address

	if utils.IsMainnet(ctx.ChainID()) {
		wrappedContractAddr = common.HexToAddress(WEVMOSContractMainnet)
	} else if utils.IsTestnet(ctx.ChainID()) {
		wrappedContractAddr = common.HexToAddress(WEVMOSContractTestnet)
	}

	// iterate over all the accounts and convert the tokens to native coins
	accountKeeper.IterateAccounts(ctx, func(account authtypes.AccountI) (stop bool) {
		ethAccount, ok := account.(*evmostypes.EthAccount)
		if !ok {
			return false
		}

		ethAddress := ethAccount.EthAddress()
		ethHexAddr := ethAddress.String()
		cosmosAddress := sdk.AccAddress(ethAddress.Bytes())

		balance, res, err := WithdrawWEVMOS(ctx, ethAddress, wrappedContractAddr, erc20Keeper)
		if err != nil {
			fmt.Println("Error withdrawing WEVMOS", err)
			logger.Debug(
				"failed to withdraw WEVMOS",
				"account", ethHexAddr,
				"balance", balance.String(),
				"error", err.Error(),
			)
		} else if res != nil && res.VmError != "" {
			fmt.Println("VM error while withdrawing WEVMOS", err)
			logger.Debug(
				"withdraw WEVMOS reverted",
				"account", ethHexAddr,
				"balance", balance.String(),
				"vm-error", res.VmError,
			)
		} else {
			fmt.Println("no error occurred")
		}

		erc20Keeper.IterateTokenPairs(ctx, func(tokenPair erc20types.TokenPair) bool {
			if !tokenPair.IsNativeCoin() {
				return false
			}

			contract := tokenPair.GetERC20Contract()

			if err := ConvertERC20Token(ctx, ethAddress, contract, cosmosAddress, erc20Keeper); err != nil {
				logger.Debug(
					"failed to convert ERC20 to native Coin",
					"account", ethHexAddr,
					"erc20", contract.String(),
					"balance", balance.String(),
					"error", err.Error(),
				)
			}

			return false
		})

		return false
	})

	erc20ModuleAccountAddress := authtypes.NewModuleAddress(erc20types.ModuleName)
	balances := bankKeeper.GetAllBalances(ctx, erc20ModuleAccountAddress)
	if balances.IsZero() {
		return nil
	}

	// burn all the coins left in the module account
	return bankKeeper.BurnCoins(ctx, erc20types.ModuleName, balances)
}

func WithdrawWEVMOS(ctx sdk.Context, from, wevmosContract common.Address, erc20Keeper erc20keeper.Keeper) (*big.Int, *evmtypes.MsgEthereumTxResponse, error) {
	balance := erc20Keeper.BalanceOf(ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI, wevmosContract, from)
	if balance == nil {
		return nil, nil, fmt.Errorf("failed to get WEVMOS balance for %s", from.String())
	}

	// only execute the withdrawal if balance is positive
	if balance.Cmp(common.Big0) < 1 {
		return nil, nil, nil
	}

	// call withdraw method from the account
	var data []byte
	res, err := erc20Keeper.CallEVMWithData(ctx, from, &wevmosContract, data, true)
	return balance, res, err
}

func ConvertERC20Token(ctx sdk.Context, from, contract common.Address, receiver sdk.AccAddress, erc20Keeper erc20keeper.Keeper) error {
	balance := erc20Keeper.BalanceOf(ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI, contract, from)
	if balance.Cmp(common.Big0) <= 0 {
		return nil
	}

	msg := erc20types.NewMsgConvertERC20(sdk.NewIntFromBigInt(balance), receiver, contract, from)

	_, err := erc20Keeper.ConvertERC20(sdk.WrapSDKContext(ctx), msg)
	if err != nil {
		return err
	}

	return nil
}
