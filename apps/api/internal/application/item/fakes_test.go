package item_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

// errRepository はrepositoryの技術的失敗を模したerror。
var errRepository = errors.New("repository failure")

// fixedClock は固定時刻を返す。
type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time { return c.now }

// sequentialPublicIDGenerator は決定的なUUIDを返す。
type sequentialPublicIDGenerator struct {
	mutex   sync.Mutex
	counter int
	failing bool
}

func (g *sequentialPublicIDGenerator) NewPublicID() (uuid.UUID, error) {
	if g.failing {
		return uuid.Nil, errors.New("id generation failed")
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.counter++

	var generated uuid.UUID
	generated[14] = byte(g.counter >> 8)
	generated[15] = byte(g.counter)
	generated[6] = 0x70
	generated[8] = 0x80
	return generated, nil
}

// fakeCategoryRepository はmemory上でCategoryRepositoryを実装する。
type fakeCategoryRepository struct {
	mutex            sync.Mutex
	categoriesByUser map[domainauth.UserID][]domaincategory.Category
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categoriesByUser: make(map[domainauth.UserID][]domaincategory.Category),
	}
}

// add はtest用にカテゴリーを登録する。
func (r *fakeCategoryRepository) add(category domaincategory.Category) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.categoriesByUser[category.UserID()] = append(
		r.categoriesByUser[category.UserID()], category)
}

func (r *fakeCategoryRepository) CreateAll(
	_ context.Context,
	categories []domaincategory.Category,
) ([]domaincategory.Category, error) {
	for _, category := range categories {
		r.add(category)
	}
	return categories, nil
}

func (r *fakeCategoryRepository) ListActiveByUserID(
	_ context.Context,
	userID domainauth.UserID,
) ([]domaincategory.Category, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.categoriesByUser[userID], nil
}

func (r *fakeCategoryRepository) FindActiveByPublicID(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domaincategory.Category, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, category := range r.categoriesByUser[userID] {
		if category.PublicID() == publicID {
			return category, nil
		}
	}
	return domaincategory.Category{}, domaincategory.ErrCategoryNotFound
}

var _ domaincategory.CategoryRepository = (*fakeCategoryRepository)(nil)

// fakeTagRepository はmemory上でTagRepositoryを実装する。
type fakeTagRepository struct {
	mutex      sync.Mutex
	tagsByUser map[domainauth.UserID][]domaintag.Tag
}

func newFakeTagRepository() *fakeTagRepository {
	return &fakeTagRepository{tagsByUser: make(map[domainauth.UserID][]domaintag.Tag)}
}

func (r *fakeTagRepository) add(tag domaintag.Tag) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.tagsByUser[tag.UserID()] = append(r.tagsByUser[tag.UserID()], tag)
}

func (r *fakeTagRepository) Create(
	_ context.Context,
	tag domaintag.Tag,
) (domaintag.Tag, error) {
	r.add(tag)
	return tag, nil
}

func (r *fakeTagRepository) ListActiveWithItemCount(
	_ context.Context,
	userID domainauth.UserID,
) ([]domaintag.Summary, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	summaries := make([]domaintag.Summary, 0, len(r.tagsByUser[userID]))
	for _, tag := range r.tagsByUser[userID] {
		summaries = append(summaries, domaintag.Summary{Tag: tag})
	}
	return summaries, nil
}

func (r *fakeTagRepository) FindActiveByPublicID(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domaintag.Tag, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, tag := range r.tagsByUser[userID] {
		if tag.PublicID() == publicID {
			return tag, nil
		}
	}
	return domaintag.Tag{}, domaintag.ErrTagNotFound
}

func (r *fakeTagRepository) ResolveActiveReferences(
	_ context.Context,
	userID domainauth.UserID,
	publicIDs []uuid.UUID,
) ([]domaintag.Reference, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	references := make([]domaintag.Reference, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		found := false
		for _, tag := range r.tagsByUser[userID] {
			if tag.PublicID() == publicID {
				references = append(references, tag.Reference())
				found = true
				break
			}
		}
		if !found {
			return nil, domaintag.ErrTagNotFound
		}
	}
	return references, nil
}

func (r *fakeTagRepository) Update(
	_ context.Context,
	tag domaintag.Tag,
	_ int32,
) (domaintag.Tag, error) {
	return tag, nil
}

func (r *fakeTagRepository) SoftDelete(
	_ context.Context,
	_ domainauth.UserID,
	_ uuid.UUID,
	_ int32,
	_ time.Time,
) error {
	return nil
}

func (r *fakeTagRepository) CountActiveItems(
	_ context.Context,
	_ domainauth.UserID,
	_ domaintag.TagID,
) (int64, error) {
	return 0, nil
}

var _ domaintag.TagRepository = (*fakeTagRepository)(nil)

// fakeItemRepository はmemory上でItemRepositoryを実装する。
//
// user IDによる絞り込みと楽観ロックの挙動をPostgreSQL実装と揃える。
type fakeItemRepository struct {
	mutex     sync.Mutex
	itemsByID map[uuid.UUID]domainitem.Item
	nextID    int64

	failOnCreate bool
}

func newFakeItemRepository() *fakeItemRepository {
	return &fakeItemRepository{
		itemsByID: make(map[uuid.UUID]domainitem.Item),
		nextID:    1,
	}
}

func (r *fakeItemRepository) Create(
	_ context.Context,
	item domainitem.Item,
) (domainitem.Item, error) {
	if r.failOnCreate {
		return domainitem.Item{}, errRepository
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored := item.WithID(domainitem.ItemID(r.nextID))
	r.nextID++
	r.itemsByID[stored.PublicID()] = stored
	return stored, nil
}

func (r *fakeItemRepository) FindByPublicID(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainitem.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.find(userID, publicID)
}

// find は呼び出し元がlock済みであることを前提とする。
func (r *fakeItemRepository) find(
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainitem.Item, error) {
	stored, ok := r.itemsByID[publicID]
	// 他ユーザーのアイテムは存在しないものとして扱う (設計書 18.3)。
	if !ok || stored.UserID() != userID {
		return domainitem.Item{}, domainitem.ErrItemNotFound
	}
	return stored, nil
}

func (r *fakeItemRepository) Update(
	_ context.Context,
	item domainitem.Item,
	expectedVersion int32,
) (domainitem.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.find(item.UserID(), item.PublicID())
	if err != nil {
		return domainitem.Item{}, err
	}
	if stored.Version() != expectedVersion || stored.IsArchived() {
		return domainitem.Item{}, domainitem.ErrItemVersionConflict
	}

	r.itemsByID[item.PublicID()] = item
	return item, nil
}

func (r *fakeItemRepository) Archive(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	archivedAt time.Time,
) (domainitem.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.find(userID, publicID)
	if err != nil {
		return domainitem.Item{}, err
	}
	archived, err := stored.Archive(expectedVersion, archivedAt)
	if err != nil {
		return domainitem.Item{}, domainitem.ErrItemVersionConflict
	}

	r.itemsByID[publicID] = archived
	return archived, nil
}

func (r *fakeItemRepository) Restore(
	_ context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainitem.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	stored, err := r.find(userID, publicID)
	if err != nil {
		return domainitem.Item{}, err
	}
	restored, err := stored.Restore(expectedVersion, now)
	if err != nil {
		return domainitem.Item{}, domainitem.ErrItemVersionConflict
	}

	r.itemsByID[publicID] = restored
	return restored, nil
}

func (r *fakeItemRepository) List(
	_ context.Context,
	userID domainauth.UserID,
	criteria domainitem.ListCriteria,
) ([]domainitem.Item, error) {
	matched := r.matching(userID, criteria)

	sort.Slice(matched, func(left, right int) bool {
		if criteria.Descending {
			return matched[left].Name() > matched[right].Name()
		}
		return matched[left].Name() < matched[right].Name()
	})

	if int(criteria.Offset) >= len(matched) {
		return nil, nil
	}
	end := int(criteria.Offset) + int(criteria.Limit)
	if end > len(matched) {
		end = len(matched)
	}
	return matched[criteria.Offset:end], nil
}

func (r *fakeItemRepository) Count(
	_ context.Context,
	userID domainauth.UserID,
	criteria domainitem.ListCriteria,
) (int64, error) {
	return int64(len(r.matching(userID, criteria))), nil
}

func (r *fakeItemRepository) matching(
	userID domainauth.UserID,
	criteria domainitem.ListCriteria,
) []domainitem.Item {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	matched := make([]domainitem.Item, 0, len(r.itemsByID))
	for _, stored := range r.itemsByID {
		if stored.UserID() != userID {
			continue
		}
		if stored.IsArchived() && !criteria.IncludeArchived {
			continue
		}
		if criteria.Keyword != "" &&
			!strings.Contains(strings.ToLower(stored.Name()), strings.ToLower(criteria.Keyword)) {
			continue
		}
		if criteria.CategoryPublicID != nil &&
			stored.Category().PublicID != *criteria.CategoryPublicID {
			continue
		}
		if criteria.NecessityLevel != nil &&
			stored.Attributes().NecessityLevel != *criteria.NecessityLevel {
			continue
		}
		matched = append(matched, stored)
	}
	return matched
}

func (r *fakeItemRepository) AggregateSummary(
	_ context.Context,
	userID domainauth.UserID,
) (domainitem.SummaryTotals, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	summary := domainitem.SummaryTotals{
		ByNecessityLevelCode: map[string]domainitem.Counts{},
		ByUsageFrequencyCode: map[string]domainitem.Counts{},
	}
	countsByCategoryPublicID := map[uuid.UUID]domainitem.CategoryCounts{}
	categoryOrder := make([]uuid.UUID, 0, len(r.itemsByID))

	for _, stored := range r.itemsByID {
		// archive済みは集計へ含めない (PostgreSQL実装と揃える)。
		if stored.UserID() != userID || stored.IsArchived() {
			continue
		}

		counts := domainitem.Counts{TypeCount: 1, TotalQuantity: int64(stored.Quantity())}
		summary.Total = summary.Total.Add(counts)

		attributes := stored.Attributes()
		reference := attributes.Category
		existing, ok := countsByCategoryPublicID[reference.PublicID]
		if !ok {
			categoryOrder = append(categoryOrder, reference.PublicID)
			existing = domainitem.CategoryCounts{Category: reference}
		}
		existing.Counts = existing.Counts.Add(counts)
		countsByCategoryPublicID[reference.PublicID] = existing

		necessityCode := attributes.NecessityLevel.String()
		summary.ByNecessityLevelCode[necessityCode] =
			summary.ByNecessityLevelCode[necessityCode].Add(counts)

		frequencyCode := attributes.UsageFrequency.String()
		summary.ByUsageFrequencyCode[frequencyCode] =
			summary.ByUsageFrequencyCode[frequencyCode].Add(counts)
	}

	// mapの反復順は不定のため、testが結果を比較できるようpublicIdで整列する。
	sort.Slice(categoryOrder, func(left, right int) bool {
		return categoryOrder[left].String() < categoryOrder[right].String()
	})
	for _, publicID := range categoryOrder {
		summary.ByCategory = append(summary.ByCategory, countsByCategoryPublicID[publicID])
	}
	return summary, nil
}

var _ domainitem.ItemRepository = (*fakeItemRepository)(nil)

// fakeAuditLogRepository は記録された操作履歴を保持する。
type fakeAuditLogRepository struct {
	mutex sync.Mutex
	logs  []domainaudit.AuditLog
}

func newFakeAuditLogRepository() *fakeAuditLogRepository {
	return &fakeAuditLogRepository{}
}

func (r *fakeAuditLogRepository) Create(
	_ context.Context,
	log domainaudit.AuditLog,
) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (r *fakeAuditLogRepository) recorded() []domainaudit.AuditLog {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	copied := make([]domainaudit.AuditLog, len(r.logs))
	copy(copied, r.logs)
	return copied
}

var _ domainaudit.AuditLogRepository = (*fakeAuditLogRepository)(nil)
