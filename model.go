package orderbook

import (
	"github.com/google/btree"

	"github.com/shopspring/decimal"
)

// Side of order, either "sell" or "buy".
type Side string

const (
	// SideSell is representation of sell Side.
	SideSell Side = "sell"
	// SideBuy is representation of buy Side.
	SideBuy Side = "buy"
)

// Kind of order, either "Market" or "Limit".
// New types of order can be added.
type Kind string

const (
	// KindMarket is representation of Market order.
	KindMarket Kind = "market"
	// KindLimit is representation of Limit order.
	KindLimit Kind = "limit"
)

type Order struct {
	ID       uint64          `json:"id"`
	Side     Side            `json:"side"`
	Kind     Kind            `json:"kind"`
	Price    decimal.Decimal `json:"price"`
	Volume   decimal.Decimal `json:"volume"`
	Locked   decimal.Decimal `json:"locked"`
	Received decimal.Decimal `json:"received"`
}

type Trade struct {
	Buy    Order           `json:"buy"`
	Sell   Order           `json:"sell"`
	Amount decimal.Decimal `json:"amount"`
	Price  decimal.Decimal `json:"price"`
}

func NewTrade(buy, sell *Order, amount, price decimal.Decimal) *Trade {
	return &Trade{
		Buy:    *buy,
		Sell:   *sell,
		Amount: amount,
		Price:  price,
	}
}

// CalculateLocked calculates locked according to order side.
func CalculateLocked(amount, price decimal.Decimal, side Side) decimal.Decimal {
	switch side {
	case SideBuy:
		return amount.Mul(price)
	case SideSell:
		return amount
	default:
		return decimal.Zero
	}
}

// Less compares two orders by price & ID, respects order Side.
// Used in order-book for sorting btree of orders.
func (o *Order) Less(other btree.Item) bool {
	operand := other.(*Order)

	switch o.Side {
	case SideBuy:
		if o.Price.LessThan(operand.Price) {
			return false
		} else if o.Price.GreaterThan(operand.Price) {
			return true
		}

		return o.ID < operand.ID
	case SideSell:
		if o.Price.LessThan(operand.Price) {
			return true
		} else if o.Price.GreaterThan(operand.Price) {
			return false
		}

		return o.ID < operand.ID
	}

	return false
}
