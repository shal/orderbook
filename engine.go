package orderbook

import (
	"fmt"
	"io"
	"strings"

	"github.com/google/btree"
	"github.com/shopspring/decimal"
)

// Engine is core structure for matching engine.
type Engine struct {
	buys  *btree.BTree
	sells *btree.BTree
}

// New creates new order-book engine.
func New() *Engine {
	return &Engine{
		buys:  btree.New(32),
		sells: btree.New(32),
	}
}

func (e *Engine) matchLimit(model *Order) []Trade {
	switch model.Side {
	case SideSell:
		return e.matchLimitSide(model, e.buys)
	case SideBuy:
		return e.matchLimitSide(model, e.sells)
	}

	return nil
}

func (e *Engine) matchMarket(order *Order) ([]Trade, *Order) {
	switch order.Side {
	case SideSell:
		return e.matchMarketWithTree(e.buys, order)
	case SideBuy:
		return e.matchMarketWithTree(e.sells, order)
	}

	return nil, order
}

func (e *Engine) match(order *Order, side *btree.BTree) []Trade {
	trades := make([]Trade, 0)

	for side.Len() > 0 {
		if order.Volume.IsZero() {
			break
		}

		// At this point offer still persists in the order-book.
		other := side.Min().(*Order)
		if !ordersMatch(order, other) {
			break
		}

		trade := e.execute(order, other)
		trades = append(trades, *trade)

		if other.Volume.IsZero() {
			side.DeleteMin()
		}
	}

	return trades
}

func (e *Engine) matchMarketWithTree(side *btree.BTree, order *Order) ([]Trade, *Order) {
	if ok := estimateMarket(order, side); !ok {
		return nil, order
	}

	trades := e.match(order, side)

	if order.Volume.IsPositive() {
		return trades, order
	}

	return trades, nil
}

func (e *Engine) matchLimitSide(order *Order, side *btree.BTree) []Trade {
	trades := e.match(order, side)

	if order.Volume.IsPositive() {
		e.openLimit(order)
	}

	return trades
}

func estimateMarket(order *Order, side *btree.BTree) bool {
	var price, volume decimal.Decimal

	side.Ascend(func(i btree.Item) bool {
		other := i.(*Order)

		if volume.Add(other.Volume).GreaterThanOrEqual(order.Volume) {
			price = price.Add(CalculateLocked(order.Volume.Sub(volume), other.Price, order.Side))
			volume = volume.Add(order.Volume.Sub(volume))
			return false
		}

		volume = volume.Add(other.Volume)
		price = price.Add(CalculateLocked(other.Volume, other.Price, order.Side))

		return true
	})

	if volume.GreaterThanOrEqual(order.Volume) && price.LessThanOrEqual(order.Locked) {
		return true
	}

	return false
}

func ordersMatch(order, other *Order) bool {
	if order.Kind == KindMarket {
		return true
	}

	switch order.Side {
	case SideBuy:
		return order.Price.GreaterThanOrEqual(other.Price)
	case SideSell:
		return order.Price.LessThanOrEqual(other.Price)
	}

	return false
}

func (e *Engine) execute(order, other *Order) *Trade {
	amount := order.Volume
	if other.Volume.LessThan(order.Volume) {
		amount = other.Volume
	}

	order.Volume = order.Volume.Sub(amount)
	other.Volume = other.Volume.Sub(amount)

	price := order.Price
	if price.IsZero() {
		price = other.Price
	} else if order.ID > other.ID {
		price = other.Price
	}

	orderFunds := CalculateLocked(amount, price, order.Side)
	otherFunds := CalculateLocked(amount, price, other.Side)

	order.Locked = order.Locked.Sub(orderFunds)
	other.Locked = other.Locked.Sub(otherFunds)

	order.Received = order.Received.Add(otherFunds)
	other.Received = other.Received.Add(orderFunds)

	return trade(order, other, amount, price)
}

func trade(first, second *Order, amount, price decimal.Decimal) *Trade {
	var buy, sell *Order

	if first.Side == SideBuy {
		buy, sell = first, second
	} else {
		buy, sell = second, first
	}

	return NewTrade(buy, sell, amount, price)
}

func (e *Engine) openLimit(order *Order) {
	switch order.Side {
	case SideBuy:
		e.buys.ReplaceOrInsert(order)
	case SideSell:
		e.sells.ReplaceOrInsert(order)
	}
}

// Match matches upcoming order with orders in order-book.
func (e *Engine) Match(order *Order) ([]Trade, *Order, error) {
	switch order.Kind {
	case KindLimit:
		return e.matchLimit(order), nil, nil
	case KindMarket:
		trades, order := e.matchMarket(order)
		return trades, order, nil
	}

	return nil, nil, nil
}

// Cancel removes order from order-book.
func (e *Engine) Cancel(order *Order) *Order {
	var item btree.Item

	switch order.Side {
	case SideSell:
		item = e.sells.Delete(order).(*Order)
	case SideBuy:
		item = e.buys.Delete(order).(*Order)
	}

	if item == nil {
		return nil
	}

	return item.(*Order)
}

// String returns orders from order-book.
// Returns list of buy orders, then sell orders.
func (e *Engine) String() string {
	builder := new(strings.Builder)

	e.buys.Ascend(func(b btree.Item) bool {
		_, _ = fmt.Fprint(builder, b.(*Order), "\n")
		return true
	})

	_, _ = fmt.Fprint(builder, "\n\n")

	e.sells.Ascend(func(b btree.Item) bool {
		_, _ = fmt.Fprint(builder, b.(*Order), "\n")
		return true
	})

	return builder.String()
}

// Print prints order-book into io.Writer.
func (e *Engine) Print(w io.Writer) {
	_, _ = fmt.Fprintln(w, e.String())
}
