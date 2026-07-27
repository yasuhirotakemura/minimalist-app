//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
)

func newCategoryRepository() *repositories.PostgresqlCategoryRepository {
	return repositories.NewPostgresqlCategoryRepository(testPool)
}

func newTagRepository() *repositories.PostgresqlTagRepository {
	return repositories.NewPostgresqlTagRepository(testPool)
}

func newItemRepository() *repositories.PostgresqlItemRepository {
	return repositories.NewPostgresqlItemRepository(testPool)
}

func testInstant() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// createTestUser はuserを作成し、内部IDを返す。
func createTestUser(t *testing.T, email string) domainauth.User {
	t.Helper()

	created, err := newUserRepository().Create(
		context.Background(), newTestUser(t, email), mustPasswordHash(t, "aGFzaGhhc2hoYXNo"))
	if err != nil {
		t.Fatalf("create user %s returned error: %v", email, err)
	}
	return created
}

// createTestCategory はカテゴリーを1件作成する。
func createTestCategory(
	t *testing.T,
	userID domainauth.UserID,
	name string,
) domaincategory.Category {
	t.Helper()

	category, err := domaincategory.NewCategory(
		mustPublicID(t), userID, name, nil, 10, testInstant())
	if err != nil {
		t.Fatalf("NewCategory returned error: %v", err)
	}

	created, err := newCategoryRepository().CreateAll(
		context.Background(), []domaincategory.Category{category})
	if err != nil {
		t.Fatalf("CreateAll returned error: %v", err)
	}
	return created[0]
}

// createTestTag はタグを1件作成する。
func createTestTag(t *testing.T, userID domainauth.UserID, name string) domaintag.Tag {
	t.Helper()

	tag, err := domaintag.NewTag(mustPublicID(t), userID, name, testInstant())
	if err != nil {
		t.Fatalf("NewTag returned error: %v", err)
	}

	created, err := newTagRepository().Create(context.Background(), tag)
	if err != nil {
		t.Fatalf("Create tag returned error: %v", err)
	}
	return created
}

func testItemAttributes(
	category domaincategory.Category,
	name string,
	tags ...domaintag.Reference,
) domainitem.Attributes {
	return domainitem.Attributes{
		Name:           name,
		Category:       category.Reference(),
		Kind:           domainitem.ItemKindDurable,
		Quantity:       1,
		UnitName:       "本",
		NecessityLevel: domainitem.NecessityLevelEssential,
		UsageFrequency: domainitem.UsageFrequencyMonthly,
		Tags:           tags,
	}
}

func createTestItem(
	t *testing.T,
	userID domainauth.UserID,
	attributes domainitem.Attributes,
) domainitem.Item {
	t.Helper()

	newItem, err := domainitem.NewItem(mustPublicID(t), userID, attributes, testInstant())
	if err != nil {
		t.Fatalf("NewItem returned error: %v", err)
	}

	created, err := newItemRepository().Create(context.Background(), newItem)
	if err != nil {
		t.Fatalf("Create item returned error: %v", err)
	}
	return created
}

func TestPostgresqlItemRepository_作成と取得(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	tag := createTestTag(t, owner.ID(), "防災")

	created := createTestItem(t, owner.ID(),
		testItemAttributes(category, "折りたたみ傘", tag.Reference()))

	if created.ID().IsZero() {
		t.Fatal("created item has no internal id")
	}
	if created.Version() != 1 {
		t.Errorf("Version = %d, want 1", created.Version())
	}

	found, err := newItemRepository().FindByPublicID(ctx, owner.ID(), created.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID returned error: %v", err)
	}

	if found.Name() != "折りたたみ傘" {
		t.Errorf("Name = %q, want 折りたたみ傘", found.Name())
	}
	if found.Category().Name != "外出・携行品" {
		t.Errorf("Category.Name = %q, want 外出・携行品", found.Category().Name)
	}
	if len(found.Tags()) != 1 || found.Tags()[0].Name != "防災" {
		t.Errorf("Tags = %+v, want 防災", found.Tags())
	}
}

func TestPostgresqlItemRepository_他ユーザーのアイテムを取得できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")

	created := createTestItem(t, owner.ID(), testItemAttributes(category, "折りたたみ傘"))

	_, err := newItemRepository().FindByPublicID(ctx, intruder.ID(), created.PublicID())
	if !errors.Is(err, domainitem.ErrItemNotFound) {
		t.Fatalf("FindByPublicID error = %v, want ErrItemNotFound", err)
	}
}

func TestPostgresqlItemRepository_他ユーザーのカテゴリーは参照できない(t *testing.T) {
	truncateAll(t)

	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	intruderCategory := createTestCategory(t, intruder.ID(), "他人のカテゴリー")

	// composite foreign key (user_id, category_id) により、
	// 他ユーザーのカテゴリーを参照するinsertはDBが拒否する。
	newItem, err := domainitem.NewItem(
		mustPublicID(t),
		owner.ID(),
		testItemAttributes(intruderCategory, "折りたたみ傘"),
		testInstant(),
	)
	if err != nil {
		t.Fatalf("NewItem returned error: %v", err)
	}

	if _, err := newItemRepository().Create(context.Background(), newItem); !errors.Is(
		err, domaincategory.ErrCategoryNotFound) {
		t.Fatalf("Create error = %v, want ErrCategoryNotFound", err)
	}
}

func TestPostgresqlItemRepository_更新と楽観ロック(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	created := createTestItem(t, owner.ID(), testItemAttributes(category, "折りたたみ傘"))

	attributes := testItemAttributes(category, "長傘")
	attributes.Quantity = 3
	next, err := created.Update(attributes, created.Version(), testInstant())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, err := newItemRepository().Update(ctx, next, created.Version())
	if err != nil {
		t.Fatalf("repository Update returned error: %v", err)
	}
	if updated.Version() != created.Version()+1 {
		t.Errorf("Version = %d, want %d", updated.Version(), created.Version()+1)
	}
	if updated.Name() != "長傘" || updated.Quantity() != 3 {
		t.Errorf("updated = %q/%d, want 長傘/3", updated.Name(), updated.Quantity())
	}

	// 古いversionでの再更新は競合となる。
	if _, err := newItemRepository().Update(ctx, next, created.Version()); !errors.Is(
		err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("stale Update error = %v, want ErrItemVersionConflict", err)
	}
}

func TestPostgresqlItemRepository_他ユーザーは更新できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	intruderCategory := createTestCategory(t, intruder.ID(), "他人のカテゴリー")
	created := createTestItem(t, owner.ID(), testItemAttributes(category, "折りたたみ傘"))

	// 他ユーザーとしてarchiveを試みる。
	_, err := newItemRepository().Archive(
		ctx, intruder.ID(), created.PublicID(), created.Version(), testInstant())
	if !errors.Is(err, domainitem.ErrItemNotFound) {
		t.Fatalf("Archive error = %v, want ErrItemNotFound", err)
	}

	// archiveされていないことを確認する。
	found, err := newItemRepository().FindByPublicID(ctx, owner.ID(), created.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID returned error: %v", err)
	}
	if found.IsArchived() {
		t.Error("IsArchived = true, want false")
	}

	_ = intruderCategory
}

func TestPostgresqlItemRepository_archiveと復元(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	created := createTestItem(t, owner.ID(), testItemAttributes(category, "折りたたみ傘"))
	createTestItem(t, owner.ID(), testItemAttributes(category, "長傘"))

	archivedAt := testInstant()
	archived, err := newItemRepository().Archive(
		ctx, owner.ID(), created.PublicID(), created.Version(), archivedAt)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if !archived.IsArchived() {
		t.Fatal("IsArchived = false, want true")
	}

	criteria, err := domainitem.NewListCriteria(domainitem.ListCriteriaInput{})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}

	// 既定ではarchive済みを除外する。
	count, err := newItemRepository().Count(ctx, owner.ID(), criteria)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	withArchived, err := domainitem.NewListCriteria(
		domainitem.ListCriteriaInput{IncludeArchived: true})
	if err != nil {
		t.Fatalf("NewListCriteria returned error: %v", err)
	}
	count, err = newItemRepository().Count(ctx, owner.ID(), withArchived)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("Count (includeDeleted) = %d, want 2", count)
	}

	// version競合。
	if _, err := newItemRepository().Restore(
		ctx, owner.ID(), created.PublicID(), created.Version(), testInstant()); !errors.Is(
		err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("Restore with stale version error = %v, want ErrItemVersionConflict", err)
	}

	restored, err := newItemRepository().Restore(
		ctx, owner.ID(), created.PublicID(), archived.Version(), testInstant())
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.IsArchived() {
		t.Error("IsArchived = true, want false")
	}
}

func TestPostgresqlItemRepository_タグの付け替え(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	first := createTestTag(t, owner.ID(), "防災")
	second := createTestTag(t, owner.ID(), "通勤")

	created := createTestItem(t, owner.ID(),
		testItemAttributes(category, "折りたたみ傘", first.Reference()))

	attributes := testItemAttributes(category, "折りたたみ傘", second.Reference())
	next, err := created.Update(attributes, created.Version(), testInstant())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if _, err := newItemRepository().Update(ctx, next, created.Version()); err != nil {
		t.Fatalf("repository Update returned error: %v", err)
	}

	found, err := newItemRepository().FindByPublicID(ctx, owner.ID(), created.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID returned error: %v", err)
	}
	if len(found.Tags()) != 1 || found.Tags()[0].Name != "通勤" {
		t.Errorf("Tags = %+v, want 通勤のみ", found.Tags())
	}
}

func TestPostgresqlItemRepository_削除済みタグはresponseへ含めない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	tag := createTestTag(t, owner.ID(), "防災")
	created := createTestItem(t, owner.ID(),
		testItemAttributes(category, "折りたたみ傘", tag.Reference()))

	if err := newTagRepository().SoftDelete(
		ctx, owner.ID(), tag.PublicID(), tag.Version(), testInstant()); err != nil {
		t.Fatalf("SoftDelete returned error: %v", err)
	}

	found, err := newItemRepository().FindByPublicID(ctx, owner.ID(), created.PublicID())
	if err != nil {
		t.Fatalf("FindByPublicID returned error: %v", err)
	}
	if len(found.Tags()) != 0 {
		t.Errorf("Tags = %+v, want empty", found.Tags())
	}
}

func TestPostgresqlItemRepository_一覧のfilterとsortとpagination(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	outdoor := createTestCategory(t, owner.ID(), "外出・携行品")
	clothes := createTestCategory(t, owner.ID(), "衣類")
	intruderCategory := createTestCategory(t, intruder.ID(), "外出・携行品")
	tag := createTestTag(t, owner.ID(), "防災")

	umbrella := testItemAttributes(outdoor, "折りたたみ傘", tag.Reference())
	umbrella.Quantity = 3
	createTestItem(t, owner.ID(), umbrella)

	jacket := testItemAttributes(clothes, "ジャケット")
	jacket.Quantity = 1
	jacket.NecessityLevel = domainitem.NecessityLevelOptional
	jacket.Notes = pointerToString("雨の日に使う")
	createTestItem(t, owner.ID(), jacket)

	// 他ユーザーのアイテムは一覧へ現れない。
	createTestItem(t, intruder.ID(), testItemAttributes(intruderCategory, "折りたたみ傘"))

	testCases := map[string]struct {
		input     domainitem.ListCriteriaInput
		wantNames []string
	}{
		"条件なし": {
			input:     domainitem.ListCriteriaInput{SortKeyName: "name", Order: "asc"},
			wantNames: []string{"ジャケット", "折りたたみ傘"},
		},
		"keywordでアイテム名を検索": {
			input:     domainitem.ListCriteriaInput{Keyword: "傘"},
			wantNames: []string{"折りたたみ傘"},
		},
		"keywordでメモを検索": {
			input:     domainitem.ListCriteriaInput{Keyword: "雨の日"},
			wantNames: []string{"ジャケット"},
		},
		"keywordのワイルドカードは無効化する": {
			input:     domainitem.ListCriteriaInput{Keyword: "%"},
			wantNames: nil,
		},
		"categoryPublicIdで絞り込む": {
			input: domainitem.ListCriteriaInput{
				CategoryPublicID: pointerToUUID(clothes.PublicID()),
			},
			wantNames: []string{"ジャケット"},
		},
		"tagPublicIdで絞り込む": {
			input: domainitem.ListCriteriaInput{
				TagPublicID: pointerToUUID(tag.PublicID()),
			},
			wantNames: []string{"折りたたみ傘"},
		},
		"necessityLevelCodeで絞り込む": {
			input:     domainitem.ListCriteriaInput{NecessityLevelCode: "optional"},
			wantNames: []string{"ジャケット"},
		},
		"usageFrequencyCodeで絞り込む": {
			input:     domainitem.ListCriteriaInput{UsageFrequencyCode: "never"},
			wantNames: nil,
		},
		"数量の降順で並べる": {
			input:     domainitem.ListCriteriaInput{SortKeyName: "quantity", Order: "desc"},
			wantNames: []string{"折りたたみ傘", "ジャケット"},
		},
		"数量の昇順で並べる": {
			input:     domainitem.ListCriteriaInput{SortKeyName: "quantity", Order: "asc"},
			wantNames: []string{"ジャケット", "折りたたみ傘"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			criteria, err := domainitem.NewListCriteria(testCase.input)
			if err != nil {
				t.Fatalf("NewListCriteria returned error: %v", err)
			}

			found, err := newItemRepository().List(ctx, owner.ID(), criteria)
			if err != nil {
				t.Fatalf("List returned error: %v", err)
			}

			names := make([]string, 0, len(found))
			for _, item := range found {
				names = append(names, item.Name())
			}
			if len(names) != len(testCase.wantNames) {
				t.Fatalf("names = %v, want %v", names, testCase.wantNames)
			}
			for index := range names {
				if names[index] != testCase.wantNames[index] {
					t.Fatalf("names = %v, want %v", names, testCase.wantNames)
				}
			}
		})
	}

	t.Run("pagination", func(t *testing.T) {
		criteria, err := domainitem.NewListCriteria(domainitem.ListCriteriaInput{
			SortKeyName: "name",
			Order:       "asc",
			Limit:       pointerToInt32(1),
			Offset:      pointerToInt32(1),
		})
		if err != nil {
			t.Fatalf("NewListCriteria returned error: %v", err)
		}

		found, err := newItemRepository().List(ctx, owner.ID(), criteria)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(found) != 1 || found[0].Name() != "折りたたみ傘" {
			t.Fatalf("found = %+v, want 折りたたみ傘のみ", found)
		}

		count, err := newItemRepository().Count(ctx, owner.ID(), criteria)
		if err != nil {
			t.Fatalf("Count returned error: %v", err)
		}
		if count != 2 {
			t.Errorf("Count = %d, want 2", count)
		}
	})
}

func TestPostgresqlItemRepository_ダッシュボード集計を返す(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	outing := createTestCategory(t, owner.ID(), "外出・携行品")
	clothes := createTestCategory(t, owner.ID(), "衣類")

	umbrella := testItemAttributes(outing, "折りたたみ傘")
	umbrella.Quantity = 2
	createTestItem(t, owner.ID(), umbrella)

	jacket := testItemAttributes(clothes, "ジャケット")
	jacket.Quantity = 1
	jacket.NecessityLevel = domainitem.NecessityLevelOptional
	jacket.UsageFrequency = domainitem.UsageFrequencyDaily
	createTestItem(t, owner.ID(), jacket)

	// archive済みは集計へ含めない。
	archivedTarget := createTestItem(
		t, owner.ID(), testItemAttributes(outing, "使わない傘"))
	if _, err := newItemRepository().Archive(
		ctx, owner.ID(), archivedTarget.PublicID(),
		archivedTarget.Version(), testInstant()); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	// 他ユーザーのアイテムも集計へ含めない。
	intruderCategory := createTestCategory(t, intruder.ID(), "外出・携行品")
	createTestItem(t, intruder.ID(), testItemAttributes(intruderCategory, "他人の傘"))

	totals, err := newItemRepository().AggregateSummary(ctx, owner.ID())
	if err != nil {
		t.Fatalf("AggregateSummary returned error: %v", err)
	}

	if totals.Total.TypeCount != 2 {
		t.Errorf("Total.TypeCount = %d, want 2", totals.Total.TypeCount)
	}
	if totals.Total.TotalQuantity != 3 {
		t.Errorf("Total.TotalQuantity = %d, want 3", totals.Total.TotalQuantity)
	}

	// 内訳はカテゴリーの表示順で返る。sort_orderが同値のためid昇順となる。
	if len(totals.ByCategory) != 2 {
		t.Fatalf("len(ByCategory) = %d, want 2", len(totals.ByCategory))
	}
	if totals.ByCategory[0].Category.Name != "外出・携行品" {
		t.Errorf("ByCategory[0].Category.Name = %q, want 外出・携行品",
			totals.ByCategory[0].Category.Name)
	}
	if totals.ByCategory[0].Counts.TotalQuantity != 2 {
		t.Errorf("ByCategory[0].Counts.TotalQuantity = %d, want 2",
			totals.ByCategory[0].Counts.TotalQuantity)
	}

	if got := totals.ByNecessityLevelCode["essential"]; got.TypeCount != 1 {
		t.Errorf("ByNecessityLevelCode[essential] = %+v, want TypeCount 1", got)
	}
	if got := totals.ByUsageFrequencyCode["daily"]; got.TypeCount != 1 {
		t.Errorf("ByUsageFrequencyCode[daily] = %+v, want TypeCount 1", got)
	}

	// Domainの整列を通すと定義順 (daily が monthly より先) へ並ぶ。
	summary := domainitem.NewSummary(totals)
	if len(summary.ByUsageFrequency) != 2 {
		t.Fatalf("len(ByUsageFrequency) = %d, want 2", len(summary.ByUsageFrequency))
	}
	if summary.ByUsageFrequency[0].Frequency != domainitem.UsageFrequencyDaily {
		t.Errorf("ByUsageFrequency[0].Frequency = %q, want daily",
			summary.ByUsageFrequency[0].Frequency)
	}
}

func TestPostgresqlTagRepository_名称の一意制約と楽観ロック(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	other := createTestUser(t, "other@example.com")
	created := createTestTag(t, owner.ID(), "防災")

	// 同一ユーザー内での同名登録は競合となる。
	duplicate, err := domaintag.NewTag(mustPublicID(t), owner.ID(), "防災", testInstant())
	if err != nil {
		t.Fatalf("NewTag returned error: %v", err)
	}
	if _, err := newTagRepository().Create(ctx, duplicate); !errors.Is(
		err, domaintag.ErrTagNameAlreadyUsed) {
		t.Fatalf("Create duplicate error = %v, want ErrTagNameAlreadyUsed", err)
	}

	// 別ユーザーは同名を登録できる。
	createTestTag(t, other.ID(), "防災")

	// 楽観ロック競合。
	renamed, err := created.Rename("防災用品", created.Version(), testInstant())
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	if _, err := newTagRepository().Update(ctx, renamed, created.Version()+1); !errors.Is(
		err, domaintag.ErrTagVersionConflict) {
		t.Fatalf("Update with stale version error = %v, want ErrTagVersionConflict", err)
	}

	updated, err := newTagRepository().Update(ctx, renamed, created.Version())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name() != "防災用品" {
		t.Errorf("Name = %q, want 防災用品", updated.Name())
	}

	// 他ユーザーからは見えない。
	if _, err := newTagRepository().FindActiveByPublicID(
		ctx, other.ID(), created.PublicID()); !errors.Is(err, domaintag.ErrTagNotFound) {
		t.Fatalf("FindActiveByPublicID error = %v, want ErrTagNotFound", err)
	}
}

func TestPostgresqlTagRepository_soft_deleteで一覧から除外する(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	category := createTestCategory(t, owner.ID(), "外出・携行品")
	tag := createTestTag(t, owner.ID(), "防災")
	createTestItem(t, owner.ID(), testItemAttributes(category, "折りたたみ傘", tag.Reference()))

	summaries, err := newTagRepository().ListActiveWithItemCount(ctx, owner.ID())
	if err != nil {
		t.Fatalf("ListActiveWithItemCount returned error: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ItemCount != 1 {
		t.Fatalf("summaries = %+v, want 1件/付与1件", summaries)
	}

	if err := newTagRepository().SoftDelete(
		ctx, owner.ID(), tag.PublicID(), tag.Version(), testInstant()); err != nil {
		t.Fatalf("SoftDelete returned error: %v", err)
	}

	summaries, err = newTagRepository().ListActiveWithItemCount(ctx, owner.ID())
	if err != nil {
		t.Fatalf("ListActiveWithItemCount returned error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("summaries = %+v, want empty", summaries)
	}

	// 削除済みの名称は再利用できる。
	createTestTag(t, owner.ID(), "防災")
}

func TestPostgresqlTagRepository_ResolveActiveReferences(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	intruder := createTestUser(t, "intruder@example.com")
	first := createTestTag(t, owner.ID(), "防災")
	second := createTestTag(t, owner.ID(), "通勤")
	intruderTag := createTestTag(t, intruder.ID(), "他人のタグ")

	references, err := newTagRepository().ResolveActiveReferences(
		ctx, owner.ID(), []uuid.UUID{first.PublicID(), second.PublicID()})
	if err != nil {
		t.Fatalf("ResolveActiveReferences returned error: %v", err)
	}
	if len(references) != 2 {
		t.Fatalf("len(references) = %d, want 2", len(references))
	}

	// 他ユーザーのタグは解決できない。
	if _, err := newTagRepository().ResolveActiveReferences(
		ctx, owner.ID(), []uuid.UUID{intruderTag.PublicID()}); !errors.Is(
		err, domaintag.ErrTagNotFound) {
		t.Fatalf("ResolveActiveReferences error = %v, want ErrTagNotFound", err)
	}
}

func TestPostgresqlCategoryRepository_ユーザーごとに分離する(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	owner := createTestUser(t, "owner@example.com")
	other := createTestUser(t, "other@example.com")
	ownerCategory := createTestCategory(t, owner.ID(), "外出・携行品")
	createTestCategory(t, other.ID(), "衣類")

	categories, err := newCategoryRepository().ListActiveByUserID(ctx, owner.ID())
	if err != nil {
		t.Fatalf("ListActiveByUserID returned error: %v", err)
	}
	if len(categories) != 1 || categories[0].Name() != "外出・携行品" {
		t.Fatalf("categories = %+v, want 外出・携行品のみ", categories)
	}

	if _, err := newCategoryRepository().FindActiveByPublicID(
		ctx, other.ID(), ownerCategory.PublicID()); !errors.Is(
		err, domaincategory.ErrCategoryNotFound) {
		t.Fatalf("FindActiveByPublicID error = %v, want ErrCategoryNotFound", err)
	}
}

func pointerToString(value string) *string { return &value }
func pointerToInt32(value int32) *int32    { return &value }
func pointerToUUID(value uuid.UUID) *uuid.UUID {
	return &value
}
