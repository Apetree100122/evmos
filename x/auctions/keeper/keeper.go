// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package keeper

import (
	"cosmossdk.io/log"
	"cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

// Keeper of the auction store
type Keeper struct {
	storeKey      types.StoreKey
	cdc           codec.BinaryCodec
	authority     sdk.AccAddress
	bankKeeper    bankkeeper.Keeper
	accountKeeper authkeeper.AccountKeeper
}

// NewKeeper creates a new auction Keeper instance
func NewKeeper(
	storeKey types.StoreKey,
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	bankKeeper bankkeeper.Keeper,
	accountKeeper authkeeper.AccountKeeper,
) Keeper {
	return Keeper{
		storeKey:      storeKey,
		cdc:           cdc,
		authority:     authority,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
	}
}

// Logger returns a auctions-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("hooks", "auctions")
}
