package biz

import (
	"context"
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// setupImportTest builds a ModelService backed by its own in-memory database.
// Each test passes a distinct DSN so the seeded Models cannot leak between
// cases (the models_by_model_id unique index makes such leaks fatal).
func setupImportTest(t *testing.T, dsn string) (*ModelService, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.Open(t, dialect.SQLite, dsn)
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	svc := &ModelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	return svc, ctx, client
}

func createImportChannel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	name string,
	supportedModels []string,
) *ent.Channel {
	t.Helper()

	testModel := "gpt-4"
	if len(supportedModels) > 0 {
		testModel = supportedModels[0]
	}

	ch, err := client.Channel.Create().
		SetType("openai").
		SetName(name).
		SetStatus("enabled").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key"}}).
		SetSupportedModels(supportedModels).
		SetDefaultTestModel(testModel).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func createImportModel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	modelID string,
	status model.Status,
	associations []*objects.ModelAssociation,
) *ent.Model {
	t.Helper()

	m, err := client.Model.Create().
		SetDeveloper("openai").
		SetModelID(modelID).
		SetName(modelID).
		SetIcon("OpenAI").
		SetGroup("openai").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{
			LoadBalancerStrategy: objects.RoutingPolicyDefault,
			TraceStickyMode:      objects.RoutingPolicyDefault,
			Associations:         associations,
		}).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)

	return m
}

func channelModelAssoc(channelID int, upstreamModelID string, priority int) *objects.ModelAssociation {
	return &objects.ModelAssociation{
		Type:     associationTypeChannelModel,
		Priority: priority,
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: channelID,
			ModelID:   upstreamModelID,
		},
	}
}

func importMetadata(modelID string) *ImportModelMetadata {
	return &ImportModelMetadata{
		ModelID:   modelID,
		Developer: "openai",
		Name:      modelID,
		Icon:      "OpenAI",
		Group:     "openai",
		ModelCard: &objects.ModelCard{},
	}
}

// requireCodedError asserts the error carries the expected machine-readable code.
func requireCodedError(t *testing.T, err error, code xerrors.ErrorCode) *xerrors.CodedError {
	t.Helper()

	require.Error(t, err)

	coded, ok := xerrors.IsCodedError(err)
	require.True(t, ok, "expected a *xerrors.CodedError, got %T: %v", err, err)
	require.Equal(t, code, coded.Code, "unexpected error code, message: %s", coded.Message)

	return coded
}

// assocPriorities extracts the priority sequence for easy comparison.
func assocPriorities(associations []*objects.ModelAssociation) []int {
	out := make([]int, 0, len(associations))
	for _, assoc := range associations {
		out = append(out, assoc.Priority)
	}

	return out
}

//
// createImportedModel
//

func TestImportUnassociatedModels_CreateNewModel(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_create?mode=memory&_fk=0")

	ch1 := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4", "gpt-4o"})
	ch2 := createImportChannel(t, ctx, client, "Channel B", []string{"gpt-4"})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: importMetadata("gpt-4"),
			Sources: []*ImportModelSource{
				{ChannelID: ch1.ID, UpstreamModelID: "gpt-4"},
				{ChannelID: ch1.ID, UpstreamModelID: "gpt-4o"},
				{ChannelID: ch2.ID, UpstreamModelID: "gpt-4"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 0, result.Appended)
	require.Len(t, result.Models, 1)
	require.Empty(t, result.Warnings, "every source is served by its channel")

	created := result.Models[0]

	// Imported Models are enabled directly, not left at the schema default.
	require.Equal(t, model.StatusEnabled, created.Status)
	require.NotEqual(t, model.DefaultStatus, created.Status)

	// Type falls back to the schema default when metadata omits it.
	require.Equal(t, model.DefaultType, created.Type)

	require.Equal(t, "gpt-4", created.ModelID)
	require.Equal(t, "openai", created.Developer)
	require.Nil(t, created.Remark)

	require.NotNil(t, created.Settings)
	require.Equal(t, objects.RoutingPolicyDefault, created.Settings.LoadBalancerStrategy)
	require.Equal(t, objects.RoutingPolicyDefault, created.Settings.TraceStickyMode)

	associations := created.Settings.Associations
	require.Len(t, associations, 3)

	// All imported rules are channel_model: it is the only type that pins both
	// the channel and the upstream model name.
	for _, assoc := range associations {
		require.Equal(t, associationTypeChannelModel, assoc.Type)
		require.NotNil(t, assoc.ChannelModel)
	}

	// Priorities increase in source order, forming an ordered failover chain.
	require.Equal(t, []int{0, 1, 2}, assocPriorities(associations))

	require.Equal(t, ch1.ID, associations[0].ChannelModel.ChannelID)
	require.Equal(t, "gpt-4", associations[0].ChannelModel.ModelID)
	require.Equal(t, ch1.ID, associations[1].ChannelModel.ChannelID)
	require.Equal(t, "gpt-4o", associations[1].ChannelModel.ModelID)
	require.Equal(t, ch2.ID, associations[2].ChannelModel.ChannelID)
	require.Equal(t, "gpt-4", associations[2].ChannelModel.ModelID)

	// Persisted, not just returned.
	reloaded, err := client.Model.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusEnabled, reloaded.Status)
	require.Len(t, reloaded.Settings.Associations, 3)
}

func TestImportUnassociatedModels_CreateAppliesOptionalMetadata(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_create_meta?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Embeddings", []string{"text-embedding-3-small"})

	embedding := model.TypeEmbedding
	remark := "imported from unassociated list"

	meta := importMetadata("text-embedding-3-small")
	meta.Type = &embedding
	meta.Remark = &remark
	meta.ModelCard = &objects.ModelCard{Knowledge: "2024-01"}

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: meta,
			Sources:  []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "text-embedding-3-small"}},
		},
	})
	require.NoError(t, err)

	created := result.Models[0]
	require.Equal(t, model.TypeEmbedding, created.Type)
	require.NotNil(t, created.Remark)
	require.Equal(t, remark, *created.Remark)
	require.Equal(t, "2024-01", created.ModelCard.Knowledge)
	require.Equal(t, model.StatusEnabled, created.Status)
}

func TestImportUnassociatedModels_CreateExceedsAssociationLimit(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_create_limit?mode=memory&_fk=0")

	upstream := make([]string, 0, MaxModelAssociations+1)
	sources := make([]*ImportModelSource, 0, MaxModelAssociations+1)

	for i := range MaxModelAssociations + 1 {
		name := fmt.Sprintf("model-%d", i)
		upstream = append(upstream, name)
	}

	ch := createImportChannel(t, ctx, client, "Big Channel", upstream)

	for _, name := range upstream {
		sources = append(sources, &ImportModelSource{ChannelID: ch.ID, UpstreamModelID: name})
	}

	require.Len(t, sources, MaxModelAssociations+1)

	_, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: importMetadata("too-many"),
			Sources:  sources,
		},
	})

	coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
	require.Contains(t, coded.Message, "would end up with 11 association rules")
	require.Contains(t, coded.Message, "the maximum is 10")

	// Nothing was written: the limit is checked before the Create.
	count, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestImportUnassociatedModels_CreateAtAssociationLimitSucceeds(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_create_at_limit?mode=memory&_fk=0")

	upstream := make([]string, 0, MaxModelAssociations)
	for i := range MaxModelAssociations {
		upstream = append(upstream, fmt.Sprintf("model-%d", i))
	}

	ch := createImportChannel(t, ctx, client, "Exact Channel", upstream)

	sources := make([]*ImportModelSource, 0, MaxModelAssociations)
	for _, name := range upstream {
		sources = append(sources, &ImportModelSource{ChannelID: ch.ID, UpstreamModelID: name})
	}

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: importMetadata("exactly-ten"),
			Sources:  sources,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Models[0].Settings.Associations, MaxModelAssociations)
	require.Equal(t,
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		assocPriorities(result.Models[0].Settings.Associations),
	)
}

//
// appendImportedAssociations
//

func TestImportUnassociatedModels_AppendStartsAfterMaxExistingPriority(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_append?mode=memory&_fk=0")

	ch1 := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})
	ch2 := createImportChannel(t, ctx, client, "Channel B", []string{"gpt-4-mirror", "gpt-4-backup"})

	// Existing priorities are deliberately out of order so the append must use
	// max(existing)+1, not len(existing).
	target := createImportModel(t, ctx, client, "gpt-4", model.StatusEnabled, []*objects.ModelAssociation{
		channelModelAssoc(ch1.ID, "gpt-4", 3),
		channelModelAssoc(ch1.ID, "gpt-4-alias", 1),
	})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				{ChannelID: ch2.ID, UpstreamModelID: "gpt-4-mirror"},
				{ChannelID: ch2.ID, UpstreamModelID: "gpt-4-backup"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.Created)
	require.Equal(t, 1, result.Appended)

	associations := result.Models[0].Settings.Associations
	require.Len(t, associations, 4)

	// Existing rules keep their priorities; new ones continue from max+1.
	require.Equal(t, []int{3, 1, 4, 5}, assocPriorities(associations))
	require.Equal(t, "gpt-4-mirror", associations[2].ChannelModel.ModelID)
	require.Equal(t, associationTypeChannelModel, associations[2].Type)
	require.Equal(t, "gpt-4-backup", associations[3].ChannelModel.ModelID)

	// Unrelated settings survive the update.
	reloaded, err := client.Model.Get(ctx, target.ID)
	require.NoError(t, err)
	require.Len(t, reloaded.Settings.Associations, 4)
	require.Equal(t, objects.RoutingPolicyDefault, reloaded.Settings.LoadBalancerStrategy)
	require.Equal(t, objects.RoutingPolicyDefault, reloaded.Settings.TraceStickyMode)
}

func TestImportUnassociatedModels_AppendSkipsDuplicateSources(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_append_dup?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4", "gpt-4o"})

	target := createImportModel(t, ctx, client, "gpt-4", model.StatusEnabled, []*objects.ModelAssociation{
		channelModelAssoc(ch.ID, "gpt-4", 0),
	})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				// Already associated: skipped with a warning.
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4"},
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4o"},
				// Repeated within the same request: also skipped.
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4o"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Appended)

	require.Len(t, result.Warnings, 2)
	require.Contains(t, result.Warnings[0], "gpt-4 is already associated with model 'gpt-4'")
	require.Contains(t, result.Warnings[0], "skipped")
	require.Contains(t, result.Warnings[1], "gpt-4o is already associated with model 'gpt-4'")

	associations := result.Models[0].Settings.Associations
	require.Len(t, associations, 2)
	require.Equal(t, []int{0, 1}, assocPriorities(associations))
	require.Equal(t, "gpt-4o", associations[1].ChannelModel.ModelID)
}

func TestImportUnassociatedModels_AppendAllDuplicatesLeavesModelUntouched(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_append_all_dup?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4", "gpt-4o"})

	existing := []*objects.ModelAssociation{
		channelModelAssoc(ch.ID, "gpt-4", 0),
		channelModelAssoc(ch.ID, "gpt-4o", 1),
	}
	target := createImportModel(t, ctx, client, "gpt-4", model.StatusEnabled, existing)

	before, err := client.Model.Get(ctx, target.ID)
	require.NoError(t, err)

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4"},
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4o"},
			},
		},
	})
	require.NoError(t, err)

	// Appended still counts the item even though nothing changed.
	require.Equal(t, 1, result.Appended)
	require.Len(t, result.Warnings, 2)

	require.Len(t, result.Models[0].Settings.Associations, 2)
	require.Equal(t, []int{0, 1}, assocPriorities(result.Models[0].Settings.Associations))

	// No write happened: updated_at is unchanged.
	after, err := client.Model.Get(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
	require.Len(t, after.Settings.Associations, 2)
	require.Equal(t, []int{0, 1}, assocPriorities(after.Settings.Associations))
}

func TestImportUnassociatedModels_AppendExceedsAssociationLimit(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_append_limit?mode=memory&_fk=0")

	upstream := make([]string, 0, 12)
	for i := range 12 {
		upstream = append(upstream, fmt.Sprintf("model-%d", i))
	}

	ch := createImportChannel(t, ctx, client, "Channel A", upstream)

	// 8 existing + 3 added = 11 > MaxModelAssociations.
	existing := make([]*objects.ModelAssociation, 0, 8)
	for i := range 8 {
		existing = append(existing, channelModelAssoc(ch.ID, upstream[i], i))
	}

	target := createImportModel(t, ctx, client, "crowded", model.StatusEnabled, existing)

	before, err := client.Model.Get(ctx, target.ID)
	require.NoError(t, err)

	_, err = svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: upstream[8]},
				{ChannelID: ch.ID, UpstreamModelID: upstream[9]},
				{ChannelID: ch.ID, UpstreamModelID: upstream[10]},
			},
		},
	})

	coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
	require.Contains(t, coded.Message, "'crowded'")
	require.Contains(t, coded.Message, "would end up with 11 association rules")

	after, err := client.Model.Get(ctx, target.ID)
	require.NoError(t, err)
	require.Len(t, after.Settings.Associations, 8)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
}

func TestImportUnassociatedModels_AppendDuplicatesDoNotCountTowardLimit(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_append_limit_dup?mode=memory&_fk=0")

	upstream := make([]string, 0, 12)
	for i := range 12 {
		upstream = append(upstream, fmt.Sprintf("model-%d", i))
	}

	ch := createImportChannel(t, ctx, client, "Channel A", upstream)

	existing := make([]*objects.ModelAssociation, 0, 10)
	for i := range 10 {
		existing = append(existing, channelModelAssoc(ch.ID, upstream[i], i))
	}

	target := createImportModel(t, ctx, client, "full", model.StatusEnabled, existing)

	// All sources are duplicates, so len(added) == 0 and 10+0 <= 10 passes.
	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: upstream[0]},
				{ChannelID: ch.ID, UpstreamModelID: upstream[1]},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Warnings, 2)
	require.Len(t, result.Models[0].Settings.Associations, 10)
}

func TestImportUnassociatedModels_AppendClampsPriorityToMax(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_clamp?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"a", "b", "c", "d", "e"})

	// Existing priorities already sit at the ceiling the association dialog accepts.
	target := createImportModel(t, ctx, client, "clamped", model.StatusEnabled, []*objects.ModelAssociation{
		channelModelAssoc(ch.ID, "a", 9),
		channelModelAssoc(ch.ID, "b", MaxModelAssociations),
	})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: "c"},
				{ChannelID: ch.ID, UpstreamModelID: "d"},
				{ChannelID: ch.ID, UpstreamModelID: "e"},
			},
		},
	})
	require.NoError(t, err)

	associations := result.Models[0].Settings.Associations
	require.Len(t, associations, 5)

	// nextPriority would be 11, 12, 13 without clamping.
	require.Equal(t, []int{9, MaxModelAssociations, MaxModelAssociations, MaxModelAssociations, MaxModelAssociations},
		assocPriorities(associations))

	for _, assoc := range associations {
		require.LessOrEqual(t, assoc.Priority, MaxModelAssociations)
	}
}

func TestImportUnassociatedModels_AppendToModelWithoutSettings(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_nil_settings?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})

	target, err := client.Model.Create().
		SetDeveloper("openai").
		SetModelID("no-settings").
		SetName("no-settings").
		SetIcon("OpenAI").
		SetGroup("openai").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(nil).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources:       []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "gpt-4"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Appended)

	associations := result.Models[0].Settings.Associations
	require.Len(t, associations, 1)
	require.Equal(t, 0, associations[0].Priority)
}

func TestImportUnassociatedModels_TargetModelNotFound(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_target_missing?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})

	missingID := 987654

	_, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &missingID,
			Sources:       []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "gpt-4"}},
		},
	})

	coded := requireCodedError(t, err, xerrors.ErrCodeNotFound)
	require.Contains(t, coded.Message, "model with id '987654'")
}

//
// Warnings (soft validation, never blocking)
//

func TestImportUnassociatedModels_WarnsOnMissingChannel(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_warn_channel?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: importMetadata("gpt-4"),
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4"},
				{ChannelID: 424242, UpstreamModelID: "gpt-4"},
			},
		},
	})

	// A missing channel is only advisory: the import still succeeds.
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0], "channel 424242 was not found")
	require.Contains(t, result.Warnings[0], "may never match")

	// The association for the missing channel is still written.
	require.Len(t, result.Models[0].Settings.Associations, 2)
	require.Equal(t, 424242, result.Models[0].Settings.Associations[1].ChannelModel.ChannelID)
	require.Equal(t, model.StatusEnabled, result.Models[0].Status)
}

func TestImportUnassociatedModels_WarnsOnUnservedUpstreamModel(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_warn_model?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			Metadata: importMetadata("aliased"),
			Sources: []*ImportModelSource{
				{ChannelID: ch.ID, UpstreamModelID: "gpt-4"},
				{ChannelID: ch.ID, UpstreamModelID: "not-served-by-channel"},
			},
		},
	})

	// Aliasing onto an upstream name the channel does not list is legitimate,
	// so the mismatch is surfaced but not rejected.
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0], "channel 'Channel A' does not serve model 'not-served-by-channel'")

	associations := result.Models[0].Settings.Associations
	require.Len(t, associations, 2)
	require.Equal(t, []int{0, 1}, assocPriorities(associations))
	require.Equal(t, "not-served-by-channel", associations[1].ChannelModel.ModelID)
}

//
// Whole-import behaviour
//

func TestImportUnassociatedModels_EmptyItems(t *testing.T) {
	svc, ctx, _ := setupImportTest(t, "file:import_empty?mode=memory&_fk=0")

	result, err := svc.ImportUnassociatedModels(ctx, nil)
	require.Nil(t, result)

	coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
	require.Equal(t, "no models to import", coded.Message)

	result, err = svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{})
	require.Nil(t, result)
	requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
}

func TestImportUnassociatedModels_MixedCreateAndAppend(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_mixed?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4", "gpt-4o", "o3"})

	target := createImportModel(t, ctx, client, "gpt-4", model.StatusEnabled, []*objects.ModelAssociation{
		channelModelAssoc(ch.ID, "gpt-4", 0),
	})

	result, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{
			TargetModelID: &target.ID,
			Sources:       []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "gpt-4o"}},
		},
		{
			Metadata: importMetadata("o3"),
			Sources:  []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "o3"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 1, result.Appended)
	require.Len(t, result.Models, 2)
	require.Empty(t, result.Warnings)

	// Results follow input order: appended target first, created model second.
	require.Equal(t, target.ID, result.Models[0].ID)
	require.Len(t, result.Models[0].Settings.Associations, 2)
	require.Equal(t, "o3", result.Models[1].ModelID)
	require.Equal(t, model.StatusEnabled, result.Models[1].Status)
}

//
// validateImportItems
//

func TestValidateImportItems(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_validate?mode=memory&_fk=0")

	enabledModel := createImportModel(t, ctx, client, "existing-enabled", model.StatusEnabled, nil)
	// Archived models still occupy the (model_id, deleted_at) unique index, so
	// the existence check must not filter by status.
	archivedModel := createImportModel(t, ctx, client, "existing-archived", model.StatusArchived, nil)
	require.Equal(t, model.StatusArchived, archivedModel.Status)

	sources := []*ImportModelSource{{ChannelID: 1, UpstreamModelID: "gpt-4"}}

	t.Run("no items is accepted at this layer", func(t *testing.T) {
		// The empty-input guard lives in ImportUnassociatedModels, not here.
		require.NoError(t, svc.validateImportItems(ctx, nil))
		require.NoError(t, svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{}))
	})

	t.Run("item without sources", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Metadata: importMetadata("new-model")},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "each imported model requires at least one upstream model", coded.Message)
	})

	t.Run("both targetModelId and metadata", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{
				TargetModelID: &enabledModel.ID,
				Metadata:      importMetadata("new-model"),
				Sources:       sources,
			},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "each imported model requires exactly one of targetModelId or metadata", coded.Message)
	})

	t.Run("neither targetModelId nor metadata", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "each imported model requires exactly one of targetModelId or metadata", coded.Message)
	})

	t.Run("empty modelId in metadata", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Metadata: importMetadata(""), Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "modelId is required for a new model", coded.Message)
	})

	t.Run("duplicate modelId in input", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Metadata: importMetadata("dup-model"), Sources: sources},
			{Metadata: importMetadata("dup-model"), Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "duplicate modelId 'dup-model' in input", coded.Message)
	})

	t.Run("duplicate targetModelId in input", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{TargetModelID: &enabledModel.ID, Sources: sources},
			{TargetModelID: &enabledModel.ID, Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeValidationFailed)
		require.Equal(t, "duplicate target model in input", coded.Message)
	})

	t.Run("modelId already exists", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Metadata: importMetadata("existing-enabled"), Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeAlreadyExists)
		require.Contains(t, coded.Message, "existing-enabled")
	})

	t.Run("archived modelId still occupies the id", func(t *testing.T) {
		err := svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{Metadata: importMetadata("existing-archived"), Sources: sources},
		})
		coded := requireCodedError(t, err, xerrors.ErrCodeAlreadyExists)
		require.Contains(t, coded.Message, "existing-archived")
	})

	t.Run("valid mix passes", func(t *testing.T) {
		other := createImportModel(t, ctx, client, "other-target", model.StatusEnabled, nil)

		require.NoError(t, svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{TargetModelID: &enabledModel.ID, Sources: sources},
			{TargetModelID: &other.ID, Sources: sources},
			{Metadata: importMetadata("brand-new-a"), Sources: sources},
			{Metadata: importMetadata("brand-new-b"), Sources: sources},
		}))
	})

	t.Run("unknown targetModelId is not rejected here", func(t *testing.T) {
		// Target existence is checked later, by appendImportedAssociations.
		unknown := 999999
		require.NoError(t, svc.validateImportItems(ctx, []*ImportUnassociatedModelItem{
			{TargetModelID: &unknown, Sources: sources},
		}))
	})
}

func TestImportUnassociatedModels_DuplicateModelIDAcrossItems(t *testing.T) {
	svc, ctx, client := setupImportTest(t, "file:import_dup_modelid?mode=memory&_fk=0")

	ch := createImportChannel(t, ctx, client, "Channel A", []string{"gpt-4"})
	source := []*ImportModelSource{{ChannelID: ch.ID, UpstreamModelID: "gpt-4"}}

	_, err := svc.ImportUnassociatedModels(ctx, []*ImportUnassociatedModelItem{
		{Metadata: importMetadata("same-id"), Sources: source},
		{Metadata: importMetadata("same-id"), Sources: source},
	})
	requireCodedError(t, err, xerrors.ErrCodeValidationFailed)

	// Validation runs before any write, so no Model was created.
	count, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

//
// buildImportedAssociations
//

func TestBuildImportedAssociations(t *testing.T) {
	channels := map[int]*Channel{
		1: {Channel: &ent.Channel{ID: 1, Name: "Channel A", SupportedModels: []string{"a", "b", "c", "d"}}},
	}

	t.Run("fresh model gets increasing priorities from zero", func(t *testing.T) {
		associations, warnings := buildImportedAssociations("req", []*ImportModelSource{
			{ChannelID: 1, UpstreamModelID: "a"},
			{ChannelID: 1, UpstreamModelID: "b"},
			{ChannelID: 1, UpstreamModelID: "c"},
		}, nil, channels)

		require.Empty(t, warnings)
		require.Len(t, associations, 3)
		require.Equal(t, []int{0, 1, 2}, assocPriorities(associations))

		for i, want := range []string{"a", "b", "c"} {
			require.Equal(t, associationTypeChannelModel, associations[i].Type)
			require.Equal(t, want, associations[i].ChannelModel.ModelID)
			require.Equal(t, 1, associations[i].ChannelModel.ChannelID)
		}
	})

	t.Run("append continues from max existing priority", func(t *testing.T) {
		existing := []*objects.ModelAssociation{
			channelModelAssoc(1, "a", 5),
			channelModelAssoc(1, "b", 2),
		}

		associations, warnings := buildImportedAssociations("req", []*ImportModelSource{
			{ChannelID: 1, UpstreamModelID: "c"},
			{ChannelID: 1, UpstreamModelID: "d"},
		}, existing, channels)

		require.Empty(t, warnings)
		require.Equal(t, []int{6, 7}, assocPriorities(associations))
	})

	t.Run("non channel_model existing rules do not seed the dedupe set", func(t *testing.T) {
		existing := []*objects.ModelAssociation{
			{Type: "channel_regex", Priority: 4, ChannelRegex: &objects.ChannelRegexAssociation{ChannelID: 1, Pattern: "^a$"}},
		}

		associations, warnings := buildImportedAssociations("req", []*ImportModelSource{
			{ChannelID: 1, UpstreamModelID: "a"},
		}, existing, channels)

		require.Empty(t, warnings)
		require.Len(t, associations, 1)
		require.Equal(t, 5, associations[0].Priority)
	})

	t.Run("duplicates are skipped and warned about", func(t *testing.T) {
		existing := []*objects.ModelAssociation{channelModelAssoc(1, "a", 0)}

		associations, warnings := buildImportedAssociations("my-model", []*ImportModelSource{
			{ChannelID: 1, UpstreamModelID: "a"},
			{ChannelID: 1, UpstreamModelID: "b"},
		}, existing, channels)

		require.Len(t, associations, 1)
		require.Equal(t, "b", associations[0].ChannelModel.ModelID)
		require.Equal(t, 1, associations[0].Priority)
		require.Len(t, warnings, 1)
		require.Equal(t, "a is already associated with model 'my-model' on channel 1, skipped", warnings[0])
	})

	t.Run("priorities are clamped to the maximum", func(t *testing.T) {
		existing := []*objects.ModelAssociation{channelModelAssoc(1, "a", 50)}

		associations, _ := buildImportedAssociations("req", []*ImportModelSource{
			{ChannelID: 1, UpstreamModelID: "b"},
			{ChannelID: 1, UpstreamModelID: "c"},
		}, existing, channels)

		require.Equal(t, []int{MaxModelAssociations, MaxModelAssociations}, assocPriorities(associations))
	})

	t.Run("no sources yields empty non-nil results", func(t *testing.T) {
		associations, warnings := buildImportedAssociations("req", nil, nil, channels)
		require.Empty(t, associations)
		require.NotNil(t, warnings)
		require.Empty(t, warnings)
	})
}

//
// checkUpstreamModelServed
//

func TestCheckUpstreamModelServed(t *testing.T) {
	served := &Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "Prefixed",
			SupportedModels: []string{"gpt-4"},
			Settings: &objects.ChannelSettings{
				ExtraModelPrefix: "openai",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4-latest", To: "gpt-4"},
				},
			},
		},
	}

	hidden := &Channel{
		Channel: &ent.Channel{
			ID:              2,
			Name:            "Hidden",
			SupportedModels: []string{"gpt-4"},
			Settings: &objects.ChannelSettings{
				ExtraModelPrefix:   "openai",
				HideOriginalModels: true,
			},
		},
	}

	channels := map[int]*Channel{1: served, 2: hidden}

	t.Run("missing channel", func(t *testing.T) {
		warning := checkUpstreamModelServed(&ImportModelSource{ChannelID: 99, UpstreamModelID: "gpt-4"}, channels)
		require.Equal(t, "channel 99 was not found, its rules may never match", warning)
	})

	t.Run("direct model is served", func(t *testing.T) {
		require.Empty(t, checkUpstreamModelServed(&ImportModelSource{ChannelID: 1, UpstreamModelID: "gpt-4"}, channels))
	})

	t.Run("prefixed variant is served", func(t *testing.T) {
		require.Empty(t, checkUpstreamModelServed(
			&ImportModelSource{ChannelID: 1, UpstreamModelID: "openai/gpt-4"}, channels))
	})

	t.Run("mapped variant is served", func(t *testing.T) {
		require.Empty(t, checkUpstreamModelServed(
			&ImportModelSource{ChannelID: 1, UpstreamModelID: "gpt-4-latest"}, channels))
	})

	t.Run("unknown model warns", func(t *testing.T) {
		warning := checkUpstreamModelServed(&ImportModelSource{ChannelID: 1, UpstreamModelID: "claude-3"}, channels)
		require.Equal(t,
			"channel 'Prefixed' does not serve model 'claude-3'; requests may fail unless the provider accepts it",
			warning)
	})

	t.Run("hideOriginalModels removes the direct entry", func(t *testing.T) {
		// GetModelEntries, not SupportedModels, is the oracle: the direct name is
		// hidden even though it is still in SupportedModels.
		require.NotEmpty(t, checkUpstreamModelServed(
			&ImportModelSource{ChannelID: 2, UpstreamModelID: "gpt-4"}, channels))
		require.Empty(t, checkUpstreamModelServed(
			&ImportModelSource{ChannelID: 2, UpstreamModelID: "openai/gpt-4"}, channels))
	})
}
