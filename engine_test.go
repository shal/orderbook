package orderbook_test

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/shal/orderbook"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	Orders  []*orderbook.Order
	Trades  []orderbook.Trade
	Rejects []*orderbook.Order
}

// New creates new instance of order model.
func NewOrder(id uint64, side orderbook.Side, t orderbook.Kind, price, volume decimal.Decimal) *orderbook.Order {
	var locked decimal.Decimal
	if t == orderbook.KindMarket {
		locked = volume
	} else {
		locked = orderbook.CalculateLocked(volume, price, side)
	}

	return &orderbook.Order{
		ID:     id,
		Side:   side,
		Kind:   t,
		Price:  price,
		Volume: volume,
		Locked: locked,
	}
}

func fabricateOrder(id uint64, amountStr, priceStr string, side orderbook.Side, oType orderbook.Kind) *orderbook.Order {
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		panic(err)
	}

	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		panic(err)
	}

	return NewOrder(id, side, oType, price, amount)
}

func randomPrice(min, max float64) string {
	price := min + rand.Float64()*(max-min)

	return strconv.FormatFloat(price, 'f', 2, 64)
}

func marketOrder(id uint64, amount string, side orderbook.Side) *orderbook.Order {
	return fabricateOrder(id, amount, "0.0", side, orderbook.KindMarket)
}

func limitOrder(id uint64, amount, price string, side orderbook.Side) *orderbook.Order {
	return fabricateOrder(id, amount, price, side, orderbook.KindLimit)
}

func (tCase *testCase) OrderLimit(side orderbook.Side, amount string, price string) *testCase {
	order := limitOrder(uint64(len(tCase.Orders)+1), amount, price, side)

	tCase.Orders = append(tCase.Orders, order)

	return tCase
}

func (tCase *testCase) OrderMarket(side orderbook.Side, amount, locked string) *testCase {
	order := marketOrder(uint64(len(tCase.Orders)+1), amount, side)
	order.Locked, _ = decimal.NewFromString(locked)

	tCase.Orders = append(tCase.Orders, order)

	return tCase
}

func (tCase *testCase) Trade(buyID uint64, sellID uint64, amountx, pricex string) *testCase {
	price, err := decimal.NewFromString(pricex)
	if err != nil {
		panic(err)
	}

	amount, err := decimal.NewFromString(amountx)
	if err != nil {
		panic(err)
	}

	trade := orderbook.NewTrade(
		limitOrder(buyID, amountx, pricex, orderbook.SideBuy),
		limitOrder(sellID, amountx, pricex, orderbook.SideSell),
		amount,
		price,
	)

	tCase.Trades = append(tCase.Trades, *trade)

	return tCase
}

func (tCase *testCase) Reject(id uint64, side orderbook.Side) *testCase {
	reject := marketOrder(id, "0.0", side)

	tCase.Rejects = append(tCase.Rejects, reject)

	return tCase
}

func (tCase *testCase) Assert(t *testing.T) {
	test := assert.New(t)

	book := orderbook.New()

	var trades []orderbook.Trade
	var rejects []*orderbook.Order

	for _, obj := range tCase.Orders {
		traded, reject, err := book.Match(obj)
		test.NoError(err)

		trades = append(trades, traded...)

		if reject != nil {
			rejects = append(rejects, reject)
		}
	}

	test.Len(trades, len(tCase.Trades), "trades number mismatch")

	for i := 0; i < len(trades); i++ {
		test.Equal(tCase.Trades[i].Buy.ID, trades[i].Buy.ID)
		test.Equal(tCase.Trades[i].Sell.ID, trades[i].Sell.ID)
		test.Equal(tCase.Trades[i].Amount, trades[i].Amount)
		test.Equal(tCase.Trades[i].Price, trades[i].Price)
	}

	test.Len(rejects, len(tCase.Rejects), "rejects number mismatch")

	for i := 0; i < len(rejects); i++ {
		test.Equal(tCase.Rejects[i].ID, rejects[i].ID)
		test.Equal(tCase.Rejects[i].Price, rejects[i].Price)
		test.Equal(tCase.Rejects[i].Side, rejects[i].Side)
	}
}

func TestEngine_Limit_Buy_NoMatch(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderLimit(orderbook.SideBuy, "1.0", "1000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_ExactMatch(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderLimit(orderbook.SideBuy, "1.0", "6000.0").
		Trade(2, 1, "1.0", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_SamePricePoint(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "1.5", "6000.0").
		OrderLimit(orderbook.SideSell, "2.0", "6000.1").
		OrderLimit(orderbook.SideBuy, "1.5", "6000.2").
		OrderLimit(orderbook.SideBuy, "0.5", "6000.3").
		Trade(3, 2, "1.5", "6000.1").
		Trade(4, 2, "0.5", "6000.1").
		Assert(t)
}

func TestEngine_Limit_Buy_PartialMatch(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "6000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "6000.0").
		Trade(2, 1, "0.5", "6000.0").
		Trade(3, 1, "0.5", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_PartialMatch_Add(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "0.4", "6000.0").
		OrderLimit(orderbook.SideSell, "0.5", "6000.0").
		OrderLimit(orderbook.SideBuy, "1.0", "6000.0").
		Trade(3, 1, "0.4", "6000.0").
		Trade(3, 2, "0.5", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Sell_PartialMatch_Add(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.4", "6000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "6000.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		Trade(1, 3, "0.4", "6000.0").
		Trade(2, 3, "0.5", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_PartialMatch_DifferentPrices(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "0.5", "6000.0").
		OrderLimit(orderbook.SideSell, "0.5", "6100.0").
		OrderLimit(orderbook.SideSell, "0.5", "6110.0").
		OrderLimit(orderbook.SideBuy, "1.5", "6200.0").
		Trade(4, 1, "0.5", "6000.0").
		Trade(4, 2, "0.5", "6100.0").
		Trade(4, 3, "0.5", "6110.0").
		Assert(t)
}

func TestEngine_Limit_Sell_PartialMatch_DifferentPrices(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.5", "6000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "6100.0").
		OrderLimit(orderbook.SideBuy, "0.5", "6110.0").
		OrderLimit(orderbook.SideSell, "1.5", "6000.0").
		Trade(3, 4, "0.5", "6110.0").
		Trade(2, 4, "0.5", "6100.0").
		Trade(1, 4, "0.5", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_ExactMatch_Search(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6100.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderLimit(orderbook.SideBuy, "1.0", "6000.0").
		Trade(3, 2, "1.0", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_Market(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6100.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderMarket(orderbook.SideBuy, "1.0", "6000.0").
		Trade(3, 2, "1.0", "6000.0").
		Assert(t)
}

func TestEngine_Limit_Buy_MarketNotEnoughLocked(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6200.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderMarket(orderbook.SideBuy, "1.5", "8000.0").
		Reject(3, orderbook.SideBuy).
		Assert(t)
}

func TestEngine_Limit_Buy_MaxPriceExceeded(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "1.0", "6200.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.0").
		OrderMarket(orderbook.SideBuy, "3.0", "20000.0").
		Reject(3, orderbook.SideBuy).
		Assert(t)
}

func TestEngine_Limit_Buy_Market_Reject(t *testing.T) {
	var testcase testCase

	testcase.
		OrderMarket(orderbook.SideBuy, "1.0", "1.0").
		Reject(1, orderbook.SideBuy).
		Assert(t)
}

func TestEngine_Limit_Buy_Market_RejectPartialMatch(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.5", "6000.0").
		OrderLimit(orderbook.SideSell, "1.0", "6000.1").
		OrderMarket(orderbook.SideBuy, "2.0", "61000.0").
		Reject(3, orderbook.SideBuy).
		Assert(t)
}

func TestEngine_Limit_Buy_Market_AfterPreviousPositionClosed(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "0.8", "5540.0").
		OrderLimit(orderbook.SideBuy, "0.8", "5540.0").
		Trade(2, 1, "0.8", "5540.0").
		OrderLimit(orderbook.SideSell, "0.2", "5563.0").
		OrderMarket(orderbook.SideBuy, "0.8", "100000.0").
		Reject(4, orderbook.SideBuy).
		Assert(t)
}

func TestEngine_Limit_Sell_Market_AfterPreviousPositionClosed(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.8", "5563.0").
		OrderLimit(orderbook.SideSell, "0.8", "5563.0").
		Trade(1, 2, "0.8", "5563.0").
		OrderLimit(orderbook.SideBuy, "0.2", "5540.0").
		OrderMarket(orderbook.SideSell, "0.8", "5563.0").
		Reject(4, orderbook.SideSell).
		Assert(t)
}

func TestEngine_Limit_Buy_Market_LowestPriceFirst(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "0.1", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5100.0").
		OrderMarket(orderbook.SideBuy, "0.6", "3050.0").
		Trade(3, 1, "0.1", "5000.0").
		Trade(3, 2, "0.5", "5100.0").
		Assert(t)
}

func TestEngine_Limit_Sell_Market_HighestPriceFirst(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.1", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5100.0").
		OrderMarket(orderbook.SideSell, "0.1", "0.1").
		Trade(2, 3, "0.1", "5100.0").
		Assert(t)
}

func TestEngine_Limit_Sell_Market_MinPriceExceeded(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.1", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5100.0").
		OrderMarket(orderbook.SideSell, "0.7", "0.7").
		Reject(3, orderbook.SideSell).
		Assert(t)
}

func TestEngine_MatchingLimitsOlderFirst(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		Trade(1, 3, "0.5", "5000.0").
		Trade(2, 6, "0.5", "5000.0").
		Trade(4, 7, "0.5", "5000.0").
		Trade(5, 8, "0.5", "5000.0").
		Assert(t)
}

func TestEngine_MatchingMarketBuyOlderFirst(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderLimit(orderbook.SideSell, "0.5", "5000.0").
		OrderMarket(orderbook.SideBuy, "2.0", "20000.0").
		Trade(6, 1, "0.5", "5000.0").
		Trade(6, 2, "0.5", "5000.0").
		Trade(6, 3, "0.5", "5000.0").
		Trade(6, 4, "0.5", "5000.0").
		Assert(t)
}

func TestEngine_MatchingMarketSellOlderFirst(t *testing.T) {
	var testcase testCase

	testcase.
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderLimit(orderbook.SideBuy, "0.5", "5000.0").
		OrderMarket(orderbook.SideSell, "2.0", "2.1").
		Trade(1, 6, "0.5", "5000.0").
		Trade(2, 6, "0.5", "5000.0").
		Trade(3, 6, "0.5", "5000.0").
		Trade(4, 6, "0.5", "5000.0").
		Assert(t)
}

func TestEngine_Cancel(t *testing.T) {
	book := orderbook.New()

	sell := limitOrder(1, "1.0", "1.0", orderbook.SideSell)
	buy := limitOrder(2, "1.0", "1.0", orderbook.SideBuy)

	trades, remain, err := book.Match(buy)

	assert.NoError(t, err)
	assert.Nil(t, remain)
	assert.Empty(t, trades)

	book.Cancel(buy)
	trades, remain, err = book.Match(sell)

	assert.NoError(t, err)
	assert.Nil(t, remain)
	assert.Empty(t, trades)
}

func BenchmarkOrderbook_InsertSellRandomPrice(b *testing.B) {
	benchWithPrice(b, func(i int) string {
		return randomPrice(100.0, 10000.0)
	})
}

func BenchmarkOrderbook_InsertSellAscendingPrice(b *testing.B) {
	benchWithPrice(b, func(i int) string {
		return strconv.Itoa(i+1) + ".0"
	})
}

func BenchmarkOrderbook_InsertSellDescendingPrice(b *testing.B) {
	benchWithPrice(b, func(i int) string {
		return strconv.Itoa(b.N-i) + ".0"
	})
}

func benchWithPrice(b *testing.B, priceGen func(i int) string) {
	book := orderbook.New()
	fake := make([]*orderbook.Order, b.N)

	for i := 0; i < b.N; i++ {
		price := priceGen(i)
		fake[i] = limitOrder(uint64(i+1), "100.0", price, orderbook.SideSell)
	}

	start := time.Now()

	for i := 0; i < b.N; i++ {
		_, _, err := book.Match(fake[i])
		if err != nil {
			b.Error(err)
		}
	}

	b.Logf("%0.2f orders/sec", float64(b.N)/time.Since(start).Seconds())
}
