package cosmos_test

import (
	"fmt"

	"cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/evmos/v19/app"
	cosmosante "github.com/evmos/evmos/v19/app/ante/cosmos"
	"github.com/evmos/evmos/v19/testutil"
	"github.com/evmos/evmos/v19/testutil/integration/common/factory"
	testutiltx "github.com/evmos/evmos/v19/testutil/tx"
	"github.com/evmos/evmos/v19/utils"
)

type deductFeeDecoratorTestCase struct {
	name        string
	balance     math.Int
	rewards     []math.Int
	gas         uint64
	gasPrice    *math.Int
	feeGranter  sdk.AccAddress
	checkTx     bool
	simulate    bool
	expPass     bool
	errContains string
	postCheck   func()
	setup       func()
	malleate    func()
}

func (suite *AnteTestSuite) TestDeductFeeDecorator() {
	var (
		dfd cosmosante.DeductFeeDecorator
		ctx sdk.Context
		app *app.Evmos
		// General setup
		addr, priv = testutiltx.NewAccAddressAndKey()
		// fee granter
		fgAddr, _   = testutiltx.NewAccAddressAndKey()
		initBalance = math.NewInt(1e18)
		lowGasPrice = math.NewInt(1)
		zero        = math.ZeroInt()
	)

	// Testcase definitions
	testcases := []deductFeeDecoratorTestCase{
		{
			name:        "pass - sufficient balance to pay fees",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         0,
			checkTx:     false,
			simulate:    true,
			expPass:     true,
			errContains: "",
		},
		{
			name:        "fail - zero gas limit in check tx mode",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         0,
			checkTx:     true,
			simulate:    false,
			expPass:     false,
			errContains: "must provide positive gas",
		},
		{
			name:        "fail - checkTx - insufficient funds",
			balance:     zero,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			checkTx:     true,
			simulate:    false,
			expPass:     false,
			errContains: "failed to deduct fee: spendable balance",
			postCheck: func() {
				// the balance should not have changed
				balance := app.BankKeeper.GetBalance(ctx, addr, utils.BaseDenom)
				suite.Require().Equal(zero, balance.Amount, "expected balance to be zero")

				// there should be no rewards
				rewards, err := testutil.GetTotalDelegationRewards(ctx, app.DistrKeeper, addr)
				suite.Require().NoError(err, "failed to get total delegation rewards")
				suite.Require().Empty(rewards, "expected rewards to be zero")
			},
		},
		{
			name:        "pass - insufficient funds but sufficient staking rewards",
			balance:     zero,
			rewards:     []math.Int{initBalance},
			gas:         10_000_000,
			checkTx:     false,
			simulate:    false,
			expPass:     true,
			errContains: "",
			postCheck: func() {
				// the balance should have increased
				balance := app.BankKeeper.GetBalance(ctx, addr, utils.BaseDenom)
				suite.Require().False(
					balance.Amount.IsZero(),
					"expected balance to have increased after withdrawing a surplus amount of staking rewards",
				)

				// the rewards should all have been withdrawn
				rewards, err := testutil.GetTotalDelegationRewards(ctx, app.DistrKeeper, addr)
				suite.Require().NoError(err, "failed to get total delegation rewards")
				suite.Require().Empty(rewards, "expected all rewards to be withdrawn")
			},
		},
		{
			name:        "fail - insufficient funds and insufficient staking rewards",
			balance:     math.NewInt(1e5),
			rewards:     []math.Int{math.NewInt(1e5)},
			gas:         10_000_000,
			checkTx:     false,
			simulate:    false,
			expPass:     false,
			errContains: "failed to deduct fee: spendable balance",
			postCheck: func() {
				// the balance should not have changed
				balance := app.BankKeeper.GetBalance(ctx, addr, utils.BaseDenom)
				suite.Require().Equal(math.NewInt(1e5), balance.Amount, "expected balance to be unchanged")

				// the rewards should not have changed
				rewards, err := testutil.GetTotalDelegationRewards(ctx, app.DistrKeeper, addr)
				suite.Require().NoError(err, "failed to get total delegation rewards")
				suite.Require().Equal(
					sdk.NewDecCoins(sdk.NewDecCoin(utils.BaseDenom, math.NewInt(1e5))),
					rewards,
					"expected rewards to be unchanged")
			},
		},
		{
			name:        "fail - sufficient balance to pay fees but provided fees < required fees",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			gasPrice:    &lowGasPrice,
			checkTx:     true,
			simulate:    false,
			expPass:     false,
			errContains: "insufficient fees",
			malleate: func() {
				ctx = ctx.WithMinGasPrices(
					sdk.NewDecCoins(
						sdk.NewDecCoin(utils.BaseDenom, math.NewInt(10_000)),
					),
				)
			},
		},
		{
			name:        "success - sufficient balance to pay fees & min gas prices is zero",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			gasPrice:    &lowGasPrice,
			checkTx:     true,
			simulate:    false,
			expPass:     true,
			errContains: "",
			malleate: func() {
				ctx = ctx.WithMinGasPrices(
					sdk.NewDecCoins(
						sdk.NewDecCoin(utils.BaseDenom, zero),
					),
				)
			},
		},
		{
			name:        "success - sufficient balance to pay fees (fees > required fees)",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			checkTx:     true,
			simulate:    false,
			expPass:     true,
			errContains: "",
			malleate: func() {
				ctx = ctx.WithMinGasPrices(
					sdk.NewDecCoins(
						sdk.NewDecCoin(utils.BaseDenom, math.NewInt(100)),
					),
				)
			},
		},
		{
			name:        "success - zero fees",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         100,
			gasPrice:    &zero,
			checkTx:     true,
			simulate:    false,
			expPass:     true,
			errContains: "",
			malleate: func() {
				ctx = ctx.WithMinGasPrices(
					sdk.NewDecCoins(
						sdk.NewDecCoin(utils.BaseDenom, zero),
					),
				)
			},
			postCheck: func() {
				// the tx sender balance should not have changed
				balance := app.BankKeeper.GetBalance(ctx, addr, utils.BaseDenom)
				suite.Require().Equal(initBalance, balance.Amount, "expected balance to be unchanged")
			},
		},
		{
			name:        "fail - with not authorized fee granter",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			feeGranter:  fgAddr,
			checkTx:     true,
			simulate:    false,
			expPass:     false,
			errContains: fmt.Sprintf("%s does not allow to pay fees for %s", fgAddr, addr),
		},
		{
			name:        "success - with authorized fee granter",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			feeGranter:  fgAddr,
			checkTx:     true,
			simulate:    false,
			expPass:     true,
			errContains: "",
			malleate: func() {
				// Fund the fee granter
				err := testutil.FundAccountWithBaseDenom(ctx, app.BankKeeper, fgAddr, initBalance.Int64())
				suite.Require().NoError(err)
				// grant the fees
				grant := sdk.NewCoins(sdk.NewCoin(
					utils.BaseDenom, initBalance,
				))
				err = app.FeeGrantKeeper.GrantAllowance(ctx, fgAddr, addr, &feegrant.BasicAllowance{
					SpendLimit: grant,
				})
				suite.Require().NoError(err)
			},
			postCheck: func() {
				// the tx sender balance should not have changed
				balance := app.BankKeeper.GetBalance(ctx, addr, utils.BaseDenom)
				suite.Require().Equal(initBalance, balance.Amount, "expected balance to be unchanged")
			},
		},
		{
			name:        "fail - authorized fee granter but no feegrant keeper on decorator",
			balance:     initBalance,
			rewards:     []math.Int{zero},
			gas:         10_000_000,
			feeGranter:  fgAddr,
			checkTx:     true,
			simulate:    false,
			expPass:     false,
			errContains: "fee grants are not enabled",
			malleate: func() {
				// Fund the fee granter
				err := testutil.FundAccountWithBaseDenom(ctx, app.BankKeeper, fgAddr, initBalance.Int64())
				suite.Require().NoError(err)
				// grant the fees
				grant := sdk.NewCoins(sdk.NewCoin(
					utils.BaseDenom, initBalance,
				))
				err = app.FeeGrantKeeper.GrantAllowance(ctx, fgAddr, addr, &feegrant.BasicAllowance{
					SpendLimit: grant,
				})
				suite.Require().NoError(err)

				// remove the feegrant keeper from the decorator
				dfd = cosmosante.NewDeductFeeDecorator(
					app.AccountKeeper, app.BankKeeper, app.DistrKeeper, nil, app.StakingKeeper, nil,
				)
			},
		},
	}

	// Test execution
	for _, tc := range testcases {
		suite.Run(tc.name, func() {
			var args factory.CosmosTxArgs
			// make the setup for the test case
			ctx, dfd, args = suite.setupDeductFeeDecoratorTestCase(addr, priv, tc)

			app = suite.GetNetwork().App

			if tc.malleate != nil {
				tc.malleate()
			}
			ctx = ctx.WithIsCheckTx(tc.checkTx)

			// Create a transaction out of the message
			tx, err := suite.GetTxFactory().BuildCosmosTx(priv, args)
			suite.Require().NoError(err, "failed to create transaction")

			// run the ante handler
			_, err = dfd.AnteHandle(ctx, tx, tc.simulate, testutil.NextFn)

			// assert the resulting error
			if tc.expPass {
				suite.Require().NoError(err, "expected no error")
			} else {
				suite.Require().Error(err, "expected error")
				suite.Require().ErrorContains(err, tc.errContains)
			}

			// run the post check
			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}
