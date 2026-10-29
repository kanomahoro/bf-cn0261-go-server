package protocol

import (
	"fmt"
	"math"
)

// ShopStock is one purchase option of a shop card.
type ShopStock struct {
	ID           uint32
	ItemID       uint32
	Type         uint16
	DurationDays uint32
	ShowPrice    uint32
	SellPrice    uint32
}

// ShopCard is one purchasable shop item.
type ShopCard struct {
	ID           uint32
	Name         []uint16
	DisplayOrder uint32
	MainCategory uint32
	SubCategory  uint32
	SaleStartsAt float64
	SaleEndsAt   float64
	Stocks       []ShopStock
}

// ShopCategory is a shop navigation entry.
type ShopCategory struct {
	ID           uint32
	Name         []uint16
	DisplayOrder uint32
	Children     []ShopCategoryChild
}

// ShopCategoryChild is a sub-category row.
type ShopCategoryChild struct {
	ID           uint32
	Name         []uint16
	DisplayOrder uint32
	PageType     uint32
}

// ShopCatalog is the whole store front.
type ShopCatalog struct {
	Cards      []ShopCard
	Categories []ShopCategory
}

// PurchaseResult is the S813 result code.
type PurchaseResult uint32

// Purchase results, matching the client's own enumeration. Note that success is
// 1: zero means "unavailable" and makes the client report a failed purchase.
const (
	PurchaseUnavailable       PurchaseResult = 0
	PurchaseOK                PurchaseResult = 1
	PurchaseAlreadyPermanent  PurchaseResult = 11
	PurchaseDurationLimit     PurchaseResult = 17
	PurchaseCapacityLimit     PurchaseResult = 22
	PurchaseInsufficientCash  PurchaseResult = 38
	PurchaseInsufficientLucci PurchaseResult = 0xFFFFFFF3
	PurchaseFailed            PurchaseResult = 0
)

// ReadShopEntry parses C801 (EnterShop).
func ReadShopEntry(p Packet) (uint32, error) {
	if p.ID != 801 {
		return 0, fmt.Errorf("%w: expected EnterShop", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	low, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	high, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	timestamp := math.Float64frombits(uint64(high)<<32 | uint64(low))
	category, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	if math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
		return 0, fmt.Errorf("%w: shop timestamp", ErrInvalidPacket)
	}
	return category, nil
}

// ShopPurchaseRequest is C808.
type ShopPurchaseRequest struct {
	StockID   uint32
	Alternate bool
}

// ReadShopPurchase parses C808.
func ReadShopPurchase(p Packet) (ShopPurchaseRequest, error) {
	if p.ID != 808 {
		return ShopPurchaseRequest{}, fmt.Errorf("%w: expected ReqShopBuy", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	stockID, err := r.Unsigned()
	if err != nil {
		return ShopPurchaseRequest{}, err
	}
	flag, err := r.Unsigned()
	if err != nil {
		return ShopPurchaseRequest{}, err
	}
	if err := r.RequireEnd(); err != nil {
		return ShopPurchaseRequest{}, err
	}
	if flag > 1 {
		return ShopPurchaseRequest{}, fmt.Errorf("%w: shop purchase flag", ErrInvalidPacket)
	}
	return ShopPurchaseRequest{StockID: stockID, Alternate: flag != 0}, nil
}

const stocksPerCard = 3

func writeStock(w *Writer, stock ShopStock) {
	w.Unsigned(stock.ID)
	w.Unsigned(stock.ItemID)
	w.Unsigned(1)
	w.Short(stock.Type)
	w.Unsigned(stock.DurationDays)
	w.Unsigned(stock.ShowPrice)
	w.Unsigned(stock.SellPrice)
}

// CreateShopReply builds S800.
func CreateShopReply(catalog ShopCatalog, preferredCategory uint32) (Packet, error) {
	if len(catalog.Cards) > 512 || len(catalog.Categories) > 16 {
		return Packet{}, fmt.Errorf("%w: shop catalog capacity", ErrInvalidPacket)
	}
	cards := 0
	for _, card := range catalog.Cards {
		if len(card.Stocks) == 0 || len(card.Stocks) > 32 {
			return Packet{}, fmt.Errorf("%w: shop stock count", ErrInvalidPacket)
		}
		cards += (len(card.Stocks) + stocksPerCard - 1) / stocksPerCard
	}
	if cards > 512 {
		return Packet{}, fmt.Errorf("%w: shop card capacity", ErrInvalidPacket)
	}
	found := false
	for _, category := range catalog.Categories {
		if category.ID == preferredCategory {
			found = true
			break
		}
	}
	if !found {
		if len(catalog.Categories) == 0 {
			preferredCategory = 0
		} else {
			preferredCategory = catalog.Categories[0].ID
		}
	}
	timestamp := oleNow()
	w := NewWriter()
	w.Unsigned(0)
	w.Unsigned(1)
	w.Double(timestamp)
	w.Unsigned(uint32(cards))
	for _, card := range catalog.Cards {
		for offset := 0; offset < len(card.Stocks); offset += stocksPerCard {
			end := min(offset+stocksPerCard, len(card.Stocks))
			stocks := card.Stocks[offset:end]
			w.Unsigned(card.ID)
			w.Unsigned(0)
			w.String(card.Name)
			w.Unsigned(card.DisplayOrder)
			w.Unsigned(card.MainCategory)
			w.Unsigned(card.SubCategory)
			w.Unsigned(1)
			w.Double(card.SaleStartsAt)
			w.Double(card.SaleEndsAt)
			writeStock(w, stocks[0])
			w.Unsigned(uint32(len(stocks)))
			for _, stock := range stocks {
				writeStock(w, stock)
			}
		}
	}
	w.Double(timestamp)
	w.Unsigned(uint32(len(catalog.Categories)))
	for _, category := range catalog.Categories {
		w.Unsigned(category.ID)
		w.String(category.Name)
		w.Unsigned(category.DisplayOrder)
		w.Unsigned(uint32(len(category.Children)))
		for _, child := range category.Children {
			w.Unsigned(child.ID)
			w.String(child.Name)
			w.Unsigned(child.DisplayOrder)
			w.Unsigned(child.PageType)
		}
	}
	w.Unsigned(0)
	w.Unsigned(preferredCategory)
	if len(w.Payload()) > 60000 {
		return Packet{}, fmt.Errorf("%w: shop catalog requires pagination", ErrInvalidPacket)
	}
	return Packet{ID: 800, Payload: w.Payload()}, nil
}

// CreateShopPurchaseReply builds S813.
func CreateShopPurchaseReply(stockID uint32, result PurchaseResult) Packet {
	w := NewWriter()
	w.Unsigned(stockID)
	w.Unsigned(uint32(result))
	return Packet{ID: 813, Payload: w.Payload()}
}

// IsPeerAction reports whether a message id is a peer action that is relayed to
// the other room members.
func IsPeerAction(id uint16) bool {
	switch id {
	case 32779, 32780, 32781, 32782, 32785, 32810, 32811, 32813:
		return true
	}
	return false
}

// ValidPeerObject checks the slot partition encoded in an object id.
func ValidPeerObject(objectID uint32, slot uint8) bool {
	if slot >= 8 || objectID < 1000000 {
		return false
	}
	return (objectID-1000000)/100000 == uint32(slot)
}

// ReadPeerActionObject parses the peer action packets and returns the source
// object id.
func ReadPeerActionObject(p Packet, sourceSlot uint8, weaponDefs []uint32) (uint32, error) {
	if !IsPeerAction(p.ID) {
		return 0, fmt.Errorf("%w: unsupported peer action %d", ErrInvalidPacket, p.ID)
	}
	r := NewReader(p.Payload)
	var source uint32
	var err error
	if p.ID == 32810 {
		weapon, err := r.Unsigned()
		if err != nil {
			return 0, err
		}
		weaponSlot, err := r.Unsigned()
		if err != nil {
			return 0, err
		}
		if source, err = r.Unsigned(); err != nil {
			return 0, err
		}
		if !ValidPeerObject(source, sourceSlot) {
			return 0, fmt.Errorf("%w: peer object partition", ErrInvalidPacket)
		}
		if _, err := r.Unsigned(); err != nil {
			return 0, err
		}
		if int(weaponSlot) >= len(weaponDefs) || weapon == 0 || weaponDefs[weaponSlot] != weapon {
			return 0, fmt.Errorf("%w: weapon not in equipped inventory", ErrInvalidPacket)
		}
		return source, r.RequireEnd()
	}
	if source, err = r.Unsigned(); err != nil {
		return 0, err
	}
	if !ValidPeerObject(source, sourceSlot) {
		return 0, fmt.Errorf("%w: peer object partition", ErrInvalidPacket)
	}
	switch p.ID {
	case 32779, 32785:
		if _, err := r.Boolean(); err != nil {
			return 0, err
		}
	case 32780:
		if _, err := r.Unsigned(); err != nil {
			return 0, err
		}
		if _, err := r.Byte(); err != nil {
			return 0, err
		}
		if _, err := r.Byte(); err != nil {
			return 0, err
		}
	case 32781:
		bits, err := r.Unsigned()
		if err != nil {
			return 0, err
		}
		value := math.Float32frombits(bits)
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) ||
			math.Abs(float64(value)) > 10000000 {
			return 0, fmt.Errorf("%w: jump float", ErrInvalidPacket)
		}
		if _, err := r.Boolean(); err != nil {
			return 0, err
		}
	case 32811:
		if _, err := r.Unsigned(); err != nil {
			return 0, err
		}
	}
	return source, r.RequireEnd()
}

// ReadRelayReport parses C3 on the relay connection (ReportUseRelay).
func ReadRelayReport(p Packet, sourceSlot uint8, resource string) (uint32, error) {
	if p.ID != 3 {
		return 0, fmt.Errorf("%w: expected ReportUseRelay", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	version, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	enabled, err := r.Boolean()
	if err != nil {
		return 0, err
	}
	gameMap, err := r.String(256)
	if err != nil {
		return 0, err
	}
	objectID, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if !ValidPeerObject(objectID, sourceSlot) {
		return 0, fmt.Errorf("%w: relay object partition", ErrInvalidPacket)
	}
	if _, err := r.String(256); err != nil {
		return 0, err
	}
	if _, err := r.Unsigned(); err != nil {
		return 0, err
	}
	if _, err := r.Unsigned(); err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	if version != 717 || !enabled || FromU16(gameMap) != resource {
		return 0, fmt.Errorf("%w: relay report does not match the game", ErrInvalidPacket)
	}
	return objectID, nil
}

// CreateDefenseSeedReply builds S5 (sampled mitigation table).
func CreateDefenseSeedReply(characterObject uint32, defense []byte) (Packet, error) {
	if characterObject < 1000000 || characterObject >= 1800000 {
		return Packet{}, fmt.Errorf("%w: defense seed owner", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(characterObject)
	w.Unsigned(uint32(len(defense)))
	w.Bytes(defense)
	return Packet{ID: 5, Payload: w.Payload()}, nil
}

// CreateCancelSeedReply builds S6 (critical-cancel bitmap).
func CreateCancelSeedReply(characterObject uint32, words []uint32) (Packet, error) {
	if characterObject < 1000000 || characterObject >= 1800000 {
		return Packet{}, fmt.Errorf("%w: cancel seed owner", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(characterObject)
	w.Unsigned(uint32(len(words)))
	for _, word := range words {
		w.Unsigned(word)
	}
	return Packet{ID: 6, Payload: w.Payload()}, nil
}

// ValidateDefenseSeedReport checks C20/C21 against the installed tables.
func ValidateDefenseSeedReport(p Packet, defense []byte, cancel []uint32) error {
	r := NewReader(p.Payload)
	switch p.ID {
	case 20:
		count, err := r.Unsigned()
		if err != nil {
			return err
		}
		if int(count) != len(defense) {
			return fmt.Errorf("%w: defense table length", ErrInvalidPacket)
		}
		for _, expected := range defense {
			value, err := r.Byte()
			if err != nil {
				return err
			}
			if value != expected {
				return fmt.Errorf("%w: defense table mismatch", ErrInvalidPacket)
			}
		}
	case 21:
		count, err := r.Unsigned()
		if err != nil {
			return err
		}
		if int(count) != len(cancel) {
			return fmt.Errorf("%w: cancel table length", ErrInvalidPacket)
		}
		for _, expected := range cancel {
			value, err := r.Unsigned()
			if err != nil {
				return err
			}
			if value != expected {
				return fmt.Errorf("%w: cancel table mismatch", ErrInvalidPacket)
			}
		}
	default:
		return fmt.Errorf("%w: expected defense seed report", ErrInvalidPacket)
	}
	return r.RequireEnd()
}
