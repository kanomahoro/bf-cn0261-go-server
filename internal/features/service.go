package features

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// Service bundles the gameplay-facing persistence operations.
type Service struct {
	db          *store.DB
	progression world.Progression
}

// NewService builds the service.
func NewService(db *store.DB, progression world.Progression) *Service {
	return &Service{db: db, progression: progression}
}

// Progression exposes the level curve.
func (s *Service) Progression() world.Progression { return s.progression }

// ErrDatabaseUnavailable reports a service built without a database, which only
// happens in tests and during a misordered startup. Callers get an error instead
// of a nil dereference on a live connection goroutine.
var ErrDatabaseUnavailable = errors.New("features: database is not configured")

func (s *Service) ready() error {
	if s.db == nil {
		return ErrDatabaseUnavailable
	}
	return nil
}

// ErrCharacterMissing reports an account without a character.
var ErrCharacterMissing = errors.New("features: account has no character")

// Profile returns the persisted character of an account.
func (s *Service) Profile(accountID uint32) (store.Character, error) {
	if err := s.ready(); err != nil {
		return store.Character{}, err
	}
	character, err := s.db.CharacterByAccount(accountID)
	if errors.Is(err, store.ErrNotFound) {
		return character, ErrCharacterMissing
	}
	return character, err
}

// CreateCharacter validates the nickname and persists a new character.
func (s *Service) CreateCharacter(accountID uint32, nickname []uint16, characterType, starter uint32) (store.Character, error) {
	if err := s.ready(); err != nil {
		return store.Character{}, err
	}
	nickname = protocol.U16String(strings.TrimSpace(protocol.FromU16(nickname)))
	if protocol.CheckNickname(nickname) != protocol.NicknameOK {
		return store.Character{}, fmt.Errorf("nickname is invalid")
	}
	text := protocol.FromU16(nickname)
	character, err := s.db.CreateCharacter(accountID, text, characterType, starter)
	if errors.Is(err, store.ErrDuplicate) {
		return store.Character{}, err
	}
	return character, err
}

// NicknameTaken reports whether a nickname is in use.
func (s *Service) NicknameTaken(nickname []uint16) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	return s.db.NicknameTaken(protocol.FromU16(nickname))
}

// WireCharacter converts a stored character to its wire view.
func (s *Service) WireCharacter(character store.Character) protocol.Character {
	level, _, _ := s.progression.Level(character.Experience)
	return protocol.Character{
		AccountID:     character.AccountID,
		Nickname:      protocol.U16String(character.Nickname),
		Level:         level,
		CharacterType: character.CharacterType,
		Type:          character.CharacterType,
	}
}

// Stats converts stored progress to the wire statistics.
func (s *Service) Stats(character store.Character) protocol.Stats {
	level, start, next := s.progression.Level(character.Experience)
	return protocol.Stats{
		Lucci:      character.Lucci,
		Cash:       character.Cash,
		Level:      level,
		LevelStart: start,
		Experience: character.Experience,
		NextLevel:  next,
	}
}

// Items lists a character's inventory.
func (s *Service) Items(characterID uint32) ([]store.Item, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	// Reconcile worn items first, exactly like the original's inventory
	// snapshot: expired equipment is dropped, the character slot is filled and
	// gear the active character cannot wear comes off before anything is read.
	if _, err := s.db.NormalizeAppearance(characterID); err != nil {
		return nil, err
	}
	return s.db.Items(characterID)
}

// WireItems converts inventory rows to the wire representation.
func (s *Service) WireItems(items []store.Item) []protocol.InvItem {
	out := make([]protocol.InvItem, 0, len(items))
	for _, item := range items {
		wire := protocol.InvItem{
			ItemID:       item.ItemID,
			Quantity:     item.Quantity,
			EquippedSlot: uint8(item.EquippedSlot),
		}
		if item.ExpiresAt != nil {
			days := time.Until(*item.ExpiresAt).Hours() / 24
			if days < 0 {
				days = 0
			}
			wire.RemainingDays = &days
		}
		out = append(out, wire)
	}
	return out
}

// Equip equips or unequips an item.
func (s *Service) Equip(characterID, itemID uint32, equip bool) error {
	if err := s.ready(); err != nil {
		return err
	}
	if equip {
		return s.db.Equip(characterID, itemID)
	}
	return s.db.Unequip(characterID, itemID)
}

// Presets loads the weapon presets in wire form after seeding and repairing
// them, exactly like the original's ReadWeaponPresets.
func (s *Service) Presets(characterID uint32) (protocol.WeaponPresets, error) {
	if err := s.ready(); err != nil {
		return protocol.WeaponPresets{}, err
	}
	stored, err := s.db.LoadPresets(characterID)
	if err != nil {
		return protocol.WeaponPresets{}, err
	}
	return wirePresets(stored), nil
}

// wirePresets converts stored presets to the wire form. Slots stay aligned by
// index: an empty slot must remain an empty slot. Compacting the list would
// shift every later weapon one position down, and the client would show it
// under the wrong preset (or as the selected weapon).
func wirePresets(stored store.Presets) protocol.WeaponPresets {
	out := protocol.WeaponPresets{Selected: stored.Selected, AvailableCount: stored.Slots}
	out.Items = make([]uint32, len(stored.Items))
	for index, item := range stored.Items {
		if item != nil {
			out.Items[index] = *item
		}
	}
	if out.AvailableCount < 2 {
		out.AvailableCount = 2
	}
	return out
}

// SetPreset selects a weapon preset and equips the weapon it holds, following
// the original's SelectWeaponPreset. It returns the reconciled presets and
// reports whether the selection was accepted (an unavailable slot, or a weapon
// the worn character cannot use, is refused rather than stored).
func (s *Service) SetPreset(characterID uint32, index uint8) (protocol.WeaponPresets, bool, error) {
	if err := s.ready(); err != nil {
		return protocol.WeaponPresets{}, false, err
	}
	stored, accepted, err := s.db.SelectPreset(characterID, index)
	if err != nil {
		return protocol.WeaponPresets{}, false, err
	}
	if !accepted {
		return protocol.WeaponPresets{}, false, nil
	}
	return wirePresets(stored), true, nil
}

// ShopWire loads the catalog in wire form.
func (s *Service) ShopWire() (protocol.ShopCatalog, error) {
	if err := s.ready(); err != nil {
		return protocol.ShopCatalog{}, err
	}
	cards, categories, err := s.db.ShopCatalog()
	if err != nil {
		return protocol.ShopCatalog{}, err
	}
	out := protocol.ShopCatalog{}
	for _, card := range cards {
		wire := protocol.ShopCard{
			ID:           card.ItemID,
			Name:         protocol.U16String(card.Name),
			DisplayOrder: card.DisplayOrder,
			MainCategory: card.MainCategory,
			SubCategory:  card.SubCategory,
			SaleStartsAt: oleDate(card.SaleStart),
			SaleEndsAt:   oleDate(card.SaleEnd),
		}
		for _, stock := range card.Stocks {
			wire.Stocks = append(wire.Stocks, protocol.ShopStock{
				ID:           stock.ID,
				ItemID:       stock.ItemID,
				Type:         0,
				DurationDays: stock.Duration,
				ShowPrice:    stock.ShowPrice,
				SellPrice:    stock.SellPrice,
			})
		}
		if len(wire.Stocks) == 0 {
			continue
		}
		out.Cards = append(out.Cards, wire)
	}
	for _, category := range categories {
		wire := protocol.ShopCategory{
			ID:           category.ID,
			Name:         protocol.U16String(category.Name),
			DisplayOrder: category.Order,
		}
		for _, child := range category.Children {
			wire.Children = append(wire.Children, protocol.ShopCategoryChild{
				ID:           child.ID,
				Name:         protocol.U16String(child.Name),
				DisplayOrder: child.Order,
				PageType:     child.PageType,
			})
		}
		out.Categories = append(out.Categories, wire)
	}
	return out, nil
}

func oleDate(value time.Time) float64 {
	return 25569.0 + float64(value.UTC().UnixNano())/1e9/86400.0
}

// purchasableStock finds the stock a purchase refers to. Cards the client only
// previews carry no buyable stock, mirroring NOT EXISTS (SELECT 1 FROM
// shop_preview_items ...) in the original's purchase statement.
func purchasableStock(cards []store.ShopCard, stockID uint32) (*store.ShopStock, uint32) {
	for i := range cards {
		if cards[i].Preview {
			continue
		}
		for j := range cards[i].Stocks {
			if cards[i].Stocks[j].ID == stockID {
				return &cards[i].Stocks[j], cards[i].ItemID
			}
		}
	}
	return nil, 0
}

// Purchase buys a stock option and returns the wire result.
func (s *Service) Purchase(characterID, stockID uint32) (protocol.PurchaseResult, error) {
	if err := s.ready(); err != nil {
		return protocol.PurchaseFailed, err
	}
	cards, _, err := s.db.ShopCatalog()
	if err != nil {
		return protocol.PurchaseFailed, err
	}
	stock, itemID := purchasableStock(cards, stockID)
	if stock == nil {
		return protocol.PurchaseUnavailable, nil
	}
	// Currency decides which "insufficient" code the client shows.
	lucci, cashBalance, err := s.db.Balance(characterID)
	if err != nil {
		return protocol.PurchaseFailed, err
	}
	useCash := stock.Currency == "cash_cn"
	if !useCash && lucci < stock.SellPrice {
		return protocol.PurchaseInsufficientLucci, nil
	}
	if useCash && cashBalance < stock.SellPrice {
		return protocol.PurchaseInsufficientCash, nil
	}
	if err := s.db.PurchaseItem(characterID, stockID, itemID, stock.SellPrice, stock.Duration, useCash); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if useCash {
				return protocol.PurchaseInsufficientCash, nil
			}
			return protocol.PurchaseInsufficientLucci, nil
		}
		return protocol.PurchaseFailed, err
	}
	return protocol.PurchaseOK, nil
}

// SaveResult persists a finished match. Retries are handled by the round
// coordinator so the cadence matches the original server.
func (s *Service) SaveResult(result store.Result) error {
	if err := s.ready(); err != nil {
		return err
	}
	return s.db.SaveResult(result, s.progression.Level)
}
