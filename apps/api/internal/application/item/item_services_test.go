package item_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	applicationitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/item"
	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

var (
	testNow    = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	ownerID    = domainauth.UserID(1)
	intruderID = domainauth.UserID(2)
)

func pointerTo[T any](value T) *T { return &value }

// fixture はitemユースケースのtest環境を組み立てる。
type fixture struct {
	items      *fakeItemRepository
	categories *fakeCategoryRepository
	tags       *fakeTagRepository
	auditLogs  *fakeAuditLogRepository
	clock      *fixedClock

	createItem          *applicationitem.CreateItemService
	updateItem          *applicationitem.UpdateItemService
	getItem             *applicationitem.GetItemService
	listItems           *applicationitem.ListItemsService
	archiveItem         *applicationitem.ArchiveItemService
	restoreItem         *applicationitem.RestoreItemService
	getDashboardSummary *applicationitem.GetDashboardSummaryService

	ownerCategory domaincategory.Category
	ownerTag      domaintag.Tag
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	items := newFakeItemRepository()
	categories := newFakeCategoryRepository()
	tags := newFakeTagRepository()
	auditLogs := newFakeAuditLogRepository()
	systemClock := &fixedClock{now: testNow}
	publicIDGenerator := &sequentialPublicIDGenerator{}
	transactionManager := transaction.NewPassthroughManager()

	dependencies := applicationitem.Dependencies{
		Items:      items,
		Categories: categories,
		Tags:       tags,
		AuditRecorder: applicationaudit.NewRecorder(
			auditLogs, publicIDGenerator, systemClock),
	}

	ownerCategory := newCategory(t, ownerID, "外出・携行品")
	categories.add(ownerCategory)
	// 他ユーザーにも同名のカテゴリーを用意し、user IDでの絞り込みを検証できるようにする。
	categories.add(newCategory(t, intruderID, "外出・携行品"))

	ownerTag := newTag(t, ownerID, "防災")
	tags.add(ownerTag)

	return &fixture{
		items:      items,
		categories: categories,
		tags:       tags,
		auditLogs:  auditLogs,
		clock:      systemClock,
		createItem: applicationitem.NewCreateItemService(
			dependencies, publicIDGenerator, systemClock, transactionManager),
		updateItem: applicationitem.NewUpdateItemService(
			dependencies, systemClock, transactionManager),
		getItem:   applicationitem.NewGetItemService(dependencies),
		listItems: applicationitem.NewListItemsService(dependencies),
		archiveItem: applicationitem.NewArchiveItemService(
			dependencies, systemClock, transactionManager),
		restoreItem: applicationitem.NewRestoreItemService(
			dependencies, systemClock, transactionManager),
		getDashboardSummary: applicationitem.NewGetDashboardSummaryService(dependencies),
		ownerCategory:       ownerCategory,
		ownerTag:            ownerTag,
	}
}

func newCategory(t *testing.T, userID domainauth.UserID, name string) domaincategory.Category {
	t.Helper()

	created, err := domaincategory.NewCategory(uuid.New(), userID, name, nil, 10, testNow)
	if err != nil {
		t.Fatalf("NewCategory returned error: %v", err)
	}
	return created.WithID(domaincategory.CategoryID(int64(userID)*100 + 1))
}

func newTag(t *testing.T, userID domainauth.UserID, name string) domaintag.Tag {
	t.Helper()

	created, err := domaintag.NewTag(uuid.New(), userID, name, testNow)
	if err != nil {
		t.Fatalf("NewTag returned error: %v", err)
	}
	return created.WithID(domaintag.TagID(int64(userID)*100 + 1))
}

func (f *fixture) validAttributes() applicationitem.AttributesParams {
	return applicationitem.AttributesParams{
		Name:               "折りたたみ傘",
		CategoryPublicID:   f.ownerCategory.PublicID(),
		Quantity:           1,
		UnitName:           "本",
		NecessityLevelCode: "essential",
		UsageFrequencyCode: "monthly",
	}
}

func (f *fixture) createTestItem(t *testing.T, name string) applicationitem.ItemResult {
	t.Helper()

	attributes := f.validAttributes()
	attributes.Name = name

	result, err := f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	})
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	return result.Item
}

func (f *fixture) auditActions() []domainaudit.ActionCode {
	logs := f.auditLogs.recorded()
	actions := make([]domainaudit.ActionCode, 0, len(logs))
	for _, log := range logs {
		actions = append(actions, log.Action())
	}
	return actions
}

func TestCreateItemService_正常系(t *testing.T) {
	f := newFixture(t)

	attributes := f.validAttributes()
	attributes.TagPublicIDs = []uuid.UUID{f.ownerTag.PublicID()}

	result, err := f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	})
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}

	if result.Item.Name != "折りたたみ傘" {
		t.Errorf("Name = %q, want 折りたたみ傘", result.Item.Name)
	}
	if result.Item.Version != 1 {
		t.Errorf("Version = %d, want 1", result.Item.Version)
	}
	if result.Item.Category.Name != "外出・携行品" {
		t.Errorf("Category.Name = %q, want 外出・携行品", result.Item.Category.Name)
	}
	if result.Item.NecessityLevelLabel != "必須" {
		t.Errorf("NecessityLevelLabel = %q, want 必須", result.Item.NecessityLevelLabel)
	}
	if len(result.Item.Tags) != 1 || result.Item.Tags[0].Name != "防災" {
		t.Errorf("Tags = %+v, want 防災", result.Item.Tags)
	}
	if result.Item.IsArchived {
		t.Error("IsArchived = true, want false")
	}
}

func TestCreateItemService_監査ログを記録する(t *testing.T) {
	f := newFixture(t)

	created := f.createTestItem(t, "折りたたみ傘")

	logs := f.auditLogs.recorded()
	if len(logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(logs))
	}
	if logs[0].Action() != domainaudit.ActionItemCreated {
		t.Errorf("Action = %q, want item_created", logs[0].Action())
	}
	if logs[0].TargetPublicID() == nil || *logs[0].TargetPublicID() != created.PublicID {
		t.Errorf("TargetPublicID = %v, want %v", logs[0].TargetPublicID(), created.PublicID)
	}
	if change, ok := logs[0].Changes()["name"]; !ok || change.To != "折りたたみ傘" {
		t.Errorf("changes[name] = %+v, want To=折りたたみ傘", change)
	}
}

func TestCreateItemService_他ユーザーのカテゴリーは指定できない(t *testing.T) {
	f := newFixture(t)

	// 他ユーザーのカテゴリーを取得し、そのpublicIdで登録を試みる。
	intruderCategories, err := f.categories.ListActiveByUserID(context.Background(), intruderID)
	if err != nil {
		t.Fatalf("ListActiveByUserID returned error: %v", err)
	}

	attributes := f.validAttributes()
	attributes.CategoryPublicID = intruderCategories[0].PublicID()

	_, err = f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	})
	if !errors.Is(err, domaincategory.ErrCategoryNotFound) {
		t.Fatalf("CreateItem error = %v, want ErrCategoryNotFound", err)
	}
}

func TestCreateItemService_存在しないタグは拒否する(t *testing.T) {
	f := newFixture(t)

	attributes := f.validAttributes()
	attributes.TagPublicIDs = []uuid.UUID{uuid.New()}

	_, err := f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	})
	if !errors.Is(err, domaintag.ErrTagNotFound) {
		t.Fatalf("CreateItem error = %v, want ErrTagNotFound", err)
	}
}

func TestCreateItemService_不正な入力は監査ログを残さない(t *testing.T) {
	f := newFixture(t)

	attributes := f.validAttributes()
	attributes.Name = "   "

	if _, err := f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	}); err == nil {
		t.Fatal("CreateItem returned nil error, want error")
	}

	if logs := f.auditLogs.recorded(); len(logs) != 0 {
		t.Errorf("len(logs) = %d, want 0", len(logs))
	}
}

func TestUpdateItemService_正常系(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	attributes := f.validAttributes()
	attributes.Name = "長傘"
	attributes.Quantity = 3

	result, err := f.updateItem.Execute(context.Background(), applicationitem.UpdateItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		Attributes:      attributes,
		ExpectedVersion: created.Version,
	})
	if err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}

	if result.Item.Name != "長傘" {
		t.Errorf("Name = %q, want 長傘", result.Item.Name)
	}
	if result.Item.Version != created.Version+1 {
		t.Errorf("Version = %d, want %d", result.Item.Version, created.Version+1)
	}

	actions := f.auditActions()
	if len(actions) != 2 || actions[1] != domainaudit.ActionItemUpdated {
		t.Errorf("actions = %v, want [item_created item_updated]", actions)
	}
}

func TestUpdateItemService_version不一致で競合(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	_, err := f.updateItem.Execute(context.Background(), applicationitem.UpdateItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		Attributes:      f.validAttributes(),
		ExpectedVersion: created.Version + 1,
	})
	if !errors.Is(err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("UpdateItem error = %v, want ErrItemVersionConflict", err)
	}
}

func TestUpdateItemService_他ユーザーは更新できない(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	_, err := f.updateItem.Execute(context.Background(), applicationitem.UpdateItemParams{
		UserID:          intruderID,
		PublicID:        created.PublicID,
		Attributes:      f.validAttributes(),
		ExpectedVersion: created.Version,
	})
	if !errors.Is(err, domainitem.ErrItemNotFound) {
		t.Fatalf("UpdateItem error = %v, want ErrItemNotFound", err)
	}
}

func TestUpdateItemService_差分が無ければ監査ログを残さない(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	if _, err := f.updateItem.Execute(
		context.Background(),
		applicationitem.UpdateItemParams{
			UserID:          ownerID,
			PublicID:        created.PublicID,
			Attributes:      f.validAttributes(),
			ExpectedVersion: created.Version,
		},
	); err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}

	if actions := f.auditActions(); len(actions) != 1 {
		t.Errorf("actions = %v, want only item_created", actions)
	}
}

func TestGetItemService_他ユーザーは取得できない(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	if _, err := f.getItem.Execute(context.Background(), applicationitem.GetItemParams{
		UserID:   ownerID,
		PublicID: created.PublicID,
	}); err != nil {
		t.Fatalf("GetItem returned error: %v", err)
	}

	_, err := f.getItem.Execute(context.Background(), applicationitem.GetItemParams{
		UserID:   intruderID,
		PublicID: created.PublicID,
	})
	if !errors.Is(err, domainitem.ErrItemNotFound) {
		t.Fatalf("GetItem error = %v, want ErrItemNotFound", err)
	}
}

func TestArchiveItemService_一覧から除外される(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")
	f.createTestItem(t, "長傘")

	archived, err := f.archiveItem.Execute(
		context.Background(),
		applicationitem.ArchiveItemParams{
			UserID:          ownerID,
			PublicID:        created.PublicID,
			ExpectedVersion: created.Version,
		},
	)
	if err != nil {
		t.Fatalf("ArchiveItem returned error: %v", err)
	}
	if !archived.Item.IsArchived {
		t.Error("IsArchived = false, want true")
	}
	if archived.Item.ArchivedAt == nil {
		t.Error("ArchivedAt = nil, want a timestamp")
	}

	listed, err := f.listItems.Execute(context.Background(), applicationitem.ListItemsParams{
		UserID: ownerID,
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if listed.Pagination.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", listed.Pagination.TotalCount)
	}

	withArchived, err := f.listItems.Execute(
		context.Background(),
		applicationitem.ListItemsParams{
			UserID:   ownerID,
			Criteria: domainitem.ListCriteriaInput{IncludeArchived: true},
		},
	)
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if withArchived.Pagination.TotalCount != 2 {
		t.Errorf("TotalCount (includeDeleted) = %d, want 2",
			withArchived.Pagination.TotalCount)
	}
}

func TestArchiveItemService_version不一致で競合(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	_, err := f.archiveItem.Execute(context.Background(), applicationitem.ArchiveItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		ExpectedVersion: created.Version + 1,
	})
	if !errors.Is(err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("ArchiveItem error = %v, want ErrItemVersionConflict", err)
	}
}

func TestArchiveItemService_二重archiveを拒否する(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	archived, err := f.archiveItem.Execute(
		context.Background(),
		applicationitem.ArchiveItemParams{
			UserID:          ownerID,
			PublicID:        created.PublicID,
			ExpectedVersion: created.Version,
		},
	)
	if err != nil {
		t.Fatalf("ArchiveItem returned error: %v", err)
	}

	_, err = f.archiveItem.Execute(context.Background(), applicationitem.ArchiveItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		ExpectedVersion: archived.Item.Version,
	})
	if !errors.Is(err, domainitem.ErrItemAlreadyArchived) {
		t.Fatalf("second ArchiveItem error = %v, want ErrItemAlreadyArchived", err)
	}
}

func TestRestoreItemService_正常系(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	archived, err := f.archiveItem.Execute(
		context.Background(),
		applicationitem.ArchiveItemParams{
			UserID:          ownerID,
			PublicID:        created.PublicID,
			ExpectedVersion: created.Version,
		},
	)
	if err != nil {
		t.Fatalf("ArchiveItem returned error: %v", err)
	}

	restored, err := f.restoreItem.Execute(
		context.Background(),
		applicationitem.RestoreItemParams{
			UserID:          ownerID,
			PublicID:        created.PublicID,
			ExpectedVersion: archived.Item.Version,
		},
	)
	if err != nil {
		t.Fatalf("RestoreItem returned error: %v", err)
	}
	if restored.Item.IsArchived {
		t.Error("IsArchived = true, want false")
	}

	actions := f.auditActions()
	if len(actions) != 3 || actions[2] != domainaudit.ActionItemRestored {
		t.Errorf("actions = %v, want [... item_restored]", actions)
	}
}

func TestRestoreItemService_archiveされていないアイテムは復元できない(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	_, err := f.restoreItem.Execute(context.Background(), applicationitem.RestoreItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		ExpectedVersion: created.Version,
	})
	if !errors.Is(err, domainitem.ErrItemNotArchived) {
		t.Fatalf("RestoreItem error = %v, want ErrItemNotArchived", err)
	}
}

func TestListItemsService_paginationを返す(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"傘A", "傘B", "傘C"} {
		f.createTestItem(t, name)
	}

	first, err := f.listItems.Execute(context.Background(), applicationitem.ListItemsParams{
		UserID:   ownerID,
		Criteria: domainitem.ListCriteriaInput{Limit: pointerTo(int32(2)), Order: "asc"},
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if len(first.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(first.Items))
	}
	if first.Pagination.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", first.Pagination.TotalCount)
	}
	if !first.Pagination.HasNext {
		t.Error("HasNext = false, want true")
	}

	second, err := f.listItems.Execute(context.Background(), applicationitem.ListItemsParams{
		UserID: ownerID,
		Criteria: domainitem.ListCriteriaInput{
			Limit:  pointerTo(int32(2)),
			Offset: pointerTo(int32(2)),
			Order:  "asc",
		},
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if len(second.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(second.Items))
	}
	if second.Pagination.HasNext {
		t.Error("HasNext = true, want false")
	}
}

func TestListItemsService_他ユーザーのアイテムを含めない(t *testing.T) {
	f := newFixture(t)
	f.createTestItem(t, "折りたたみ傘")

	result, err := f.listItems.Execute(context.Background(), applicationitem.ListItemsParams{
		UserID: intruderID,
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if result.Pagination.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.Pagination.TotalCount)
	}
	if len(result.Items) != 0 {
		t.Errorf("len(Items) = %d, want 0", len(result.Items))
	}
}

func TestListItemsService_不正な条件を拒否する(t *testing.T) {
	f := newFixture(t)

	_, err := f.listItems.Execute(context.Background(), applicationitem.ListItemsParams{
		UserID:   ownerID,
		Criteria: domainitem.ListCriteriaInput{SortKeyName: "score"},
	})
	if err == nil {
		t.Fatal("ListItems returned nil error, want error")
	}
}

func TestGetDashboardSummaryService_集計値を返す(t *testing.T) {
	f := newFixture(t)

	// 数量2のアイテムを1件、数量1のアイテムを1件登録する。
	attributes := f.validAttributes()
	attributes.Name = "モバイルバッテリー"
	attributes.Quantity = 2
	attributes.UsageFrequencyCode = "weekly"
	if _, err := f.createItem.Execute(context.Background(), applicationitem.CreateItemParams{
		UserID:     ownerID,
		Attributes: attributes,
	}); err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	f.createTestItem(t, "折りたたみ傘")

	result, err := f.getDashboardSummary.Execute(
		context.Background(),
		applicationitem.GetDashboardSummaryParams{UserID: ownerID},
	)
	if err != nil {
		t.Fatalf("GetDashboardSummary returned error: %v", err)
	}

	if result.Total.TypeCount != 2 {
		t.Errorf("Total.TypeCount = %d, want 2", result.Total.TypeCount)
	}
	if result.Total.TotalQuantity != 3 {
		t.Errorf("Total.TotalQuantity = %d, want 3", result.Total.TotalQuantity)
	}

	// 両方とも同一カテゴリーのため内訳は1件へまとまる。
	if len(result.CategoryBreakdown) != 1 {
		t.Fatalf("len(CategoryBreakdown) = %d, want 1", len(result.CategoryBreakdown))
	}
	if got := result.CategoryBreakdown[0].Category.Name; got != "外出・携行品" {
		t.Errorf("CategoryBreakdown[0].Category.Name = %q, want 外出・携行品", got)
	}
	if got := result.CategoryBreakdown[0].Counts.TotalQuantity; got != 3 {
		t.Errorf("CategoryBreakdown[0].Counts.TotalQuantity = %d, want 3", got)
	}

	// 使用頻度はcode体系の定義順 (weekly が monthly より先) で並ぶ。
	if len(result.UsageFrequencyBreakdown) != 2 {
		t.Fatalf("len(UsageFrequencyBreakdown) = %d, want 2", len(result.UsageFrequencyBreakdown))
	}
	if result.UsageFrequencyBreakdown[0].Code != "weekly" {
		t.Errorf("UsageFrequencyBreakdown[0].Code = %q, want weekly",
			result.UsageFrequencyBreakdown[0].Code)
	}
	if result.UsageFrequencyBreakdown[0].Label != "週に1回程度" {
		t.Errorf("UsageFrequencyBreakdown[0].Label = %q, want 週に1回程度",
			result.UsageFrequencyBreakdown[0].Label)
	}

	// 必要度は essential のみが該当し、0件の区分は返さない。
	if len(result.NecessityLevelBreakdown) != 1 {
		t.Fatalf("len(NecessityLevelBreakdown) = %d, want 1", len(result.NecessityLevelBreakdown))
	}
	if result.NecessityLevelBreakdown[0].Code != "essential" {
		t.Errorf("NecessityLevelBreakdown[0].Code = %q, want essential",
			result.NecessityLevelBreakdown[0].Code)
	}
}

func TestGetDashboardSummaryService_archive済みを含めない(t *testing.T) {
	f := newFixture(t)
	created := f.createTestItem(t, "折りたたみ傘")

	if _, err := f.archiveItem.Execute(context.Background(), applicationitem.ArchiveItemParams{
		UserID:          ownerID,
		PublicID:        created.PublicID,
		ExpectedVersion: created.Version,
	}); err != nil {
		t.Fatalf("ArchiveItem returned error: %v", err)
	}

	result, err := f.getDashboardSummary.Execute(
		context.Background(),
		applicationitem.GetDashboardSummaryParams{UserID: ownerID},
	)
	if err != nil {
		t.Fatalf("GetDashboardSummary returned error: %v", err)
	}

	if result.Total.TypeCount != 0 || result.Total.TotalQuantity != 0 {
		t.Errorf("Total = %+v, want zero", result.Total)
	}
	if len(result.CategoryBreakdown) != 0 {
		t.Errorf("len(CategoryBreakdown) = %d, want 0", len(result.CategoryBreakdown))
	}
}
