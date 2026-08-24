package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// MaxModelAssociations caps how many associations a single Model may hold.
// It mirrors the limit enforced by the association dialog so that an import
// cannot build a Model the UI is unable to edit afterwards.
const MaxModelAssociations = 10

// associationTypeChannelModel binds one upstream model on one channel. Imports
// use it exclusively: it is the only association type that pins both the
// channel and the upstream model name, so an imported rule matches exactly the
// pair the user picked.
const associationTypeChannelModel = "channel_model"

// ImportModelSource is one upstream (channel, model) pair to associate with a
// target Model. UpstreamModelID is sent to the provider as-is, so it must be a
// model the channel actually serves.
type ImportModelSource struct {
	ChannelID       int    `json:"channelId"`
	UpstreamModelID string `json:"upstreamModelId"`
}

// ImportModelMetadata carries the Model fields that the unassociated-channel
// list cannot supply. The frontend completes them from providers.json.
type ImportModelMetadata struct {
	ModelID   string             `json:"modelId"`
	Developer string             `json:"developer"`
	Type      *model.Type        `json:"type"`
	Name      string             `json:"name"`
	Icon      string             `json:"icon"`
	Group     string             `json:"group"`
	ModelCard *objects.ModelCard `json:"modelCard"`
	Remark    *string            `json:"remark"`
}

// ImportUnassociatedModelItem imports one target Model. Exactly one of
// TargetModelID (append to an existing Model) or Metadata (create a new one)
// must be set.
type ImportUnassociatedModelItem struct {
	TargetModelID *int                 `json:"targetModelId"`
	Metadata      *ImportModelMetadata `json:"metadata"`
	Sources       []*ImportModelSource `json:"sources"`
}

// ImportUnassociatedModelsResult reports what the import did. Warnings are
// advisory: they never abort the import.
type ImportUnassociatedModelsResult struct {
	Created  int          `json:"created"`
	Appended int          `json:"appended"`
	Models   []*ent.Model `json:"models"`
	Warnings []string     `json:"warnings"`
}

// ImportUnassociatedModels wires upstream channel models into AxonHub Models,
// creating new Models or appending associations to existing ones.
//
// The whole import is one unit of work: the GraphQL layer wraps every mutation
// in a transaction, so any error rolls back every item. Newly created Models
// are enabled directly, because a Model created through CreateModelInput would
// default to disabled and the router only serves enabled Models.
func (svc *ModelService) ImportUnassociatedModels(
	ctx context.Context,
	items []*ImportUnassociatedModelItem,
) (*ImportUnassociatedModelsResult, error) {
	if len(items) == 0 {
		return nil, xerrors.ValidationError("no models to import")
	}

	channels, err := svc.loadImportChannels(ctx, items)
	if err != nil {
		return nil, err
	}

	if err := svc.validateImportItems(ctx, items); err != nil {
		return nil, err
	}

	result := &ImportUnassociatedModelsResult{
		Models:   make([]*ent.Model, 0, len(items)),
		Warnings: make([]string, 0),
	}

	for _, item := range items {
		var (
			imported *ent.Model
			warnings []string
			err      error
		)

		if item.TargetModelID != nil {
			imported, warnings, err = svc.appendImportedAssociations(ctx, *item.TargetModelID, item.Sources, channels)
			if err != nil {
				return nil, err
			}

			result.Appended++
		} else {
			imported, warnings, err = svc.createImportedModel(ctx, item, channels)
			if err != nil {
				return nil, err
			}

			result.Created++
		}

		result.Models = append(result.Models, imported)
		result.Warnings = append(result.Warnings, warnings...)
	}

	return result, nil
}

// loadImportChannels fetches every channel referenced by the import, keyed by ID.
func (svc *ModelService) loadImportChannels(
	ctx context.Context,
	items []*ImportUnassociatedModelItem,
) (map[int]*Channel, error) {
	channelIDs := make([]int, 0)

	for _, item := range items {
		for _, source := range item.Sources {
			channelIDs = append(channelIDs, source.ChannelID)
		}
	}

	channelIDs = lo.Uniq(channelIDs)
	if len(channelIDs) == 0 {
		return map[int]*Channel{}, nil
	}

	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	// Wrap once: GetModelEntries caches its result on the wrapper, so reusing
	// these instances keeps the entry map from being rebuilt per source.
	return lo.SliceToMap(channels, func(ch *ent.Channel) (int, *Channel) {
		return ch.ID, &Channel{Channel: ch}
	}), nil
}

func (svc *ModelService) validateImportItems(ctx context.Context, items []*ImportUnassociatedModelItem) error {
	newModelIDs := make([]string, 0, len(items))
	seenModelIDs := make(map[string]bool, len(items))
	seenTargetIDs := make(map[int]bool, len(items))

	for _, item := range items {
		if len(item.Sources) == 0 {
			return xerrors.ValidationError("each imported model requires at least one upstream model")
		}

		if (item.TargetModelID == nil) == (item.Metadata == nil) {
			return xerrors.ValidationError("each imported model requires exactly one of targetModelId or metadata")
		}

		if item.TargetModelID != nil {
			if seenTargetIDs[*item.TargetModelID] {
				return xerrors.ValidationError("duplicate target model in input")
			}

			seenTargetIDs[*item.TargetModelID] = true

			continue
		}

		if item.Metadata.ModelID == "" {
			return xerrors.ValidationError("modelId is required for a new model")
		}

		if seenModelIDs[item.Metadata.ModelID] {
			return xerrors.ValidationError(
				fmt.Sprintf("duplicate modelId '%s' in input", item.Metadata.ModelID),
			)
		}

		seenModelIDs[item.Metadata.ModelID] = true
		newModelIDs = append(newModelIDs, item.Metadata.ModelID)
	}

	if len(newModelIDs) == 0 {
		return nil
	}

	// modelID is the global identity of a Model, matching the unique index on
	// (model_id, deleted_at). Archived models still occupy that index, so this
	// query must not filter by status.
	existing, err := svc.entFromContext(ctx).Model.Query().
		Where(model.ModelIDIn(newModelIDs...)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing models: %w", err)
	}

	if len(existing) > 0 {
		return xerrors.AlreadyExistsError(
			fmt.Sprintf("models with modelId %v", lo.Map(existing, func(m *ent.Model, _ int) string {
				return m.ModelID
			})),
		)
	}

	return nil
}

func (svc *ModelService) createImportedModel(
	ctx context.Context,
	item *ImportUnassociatedModelItem,
	channels map[int]*Channel,
) (*ent.Model, []string, error) {
	meta := item.Metadata

	associations, warnings := buildImportedAssociations(meta.ModelID, item.Sources, nil, channels)
	if len(associations) > MaxModelAssociations {
		return nil, nil, associationLimitError(meta.ModelID, len(associations))
	}

	settings := &objects.ModelSettings{
		LoadBalancerStrategy: objects.RoutingPolicyDefault,
		TraceStickyMode:      objects.RoutingPolicyDefault,
		Associations:         associations,
	}

	if err := validateModelSettings(settings); err != nil {
		return nil, nil, err
	}

	builder := svc.entFromContext(ctx).Model.Create().
		SetDeveloper(meta.Developer).
		SetModelID(meta.ModelID).
		SetName(meta.Name).
		SetIcon(meta.Icon).
		SetGroup(meta.Group).
		SetModelCard(meta.ModelCard).
		SetSettings(settings).
		// Imported models are enabled directly: the router only serves enabled
		// Models, so a disabled import would silently do nothing.
		SetStatus(model.StatusEnabled)

	if meta.Type != nil {
		builder.SetType(*meta.Type)
	}

	if meta.Remark != nil {
		builder.SetRemark(*meta.Remark)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create model: %w", err)
	}

	return created, warnings, nil
}

func (svc *ModelService) appendImportedAssociations(
	ctx context.Context,
	targetModelID int,
	sources []*ImportModelSource,
	channels map[int]*Channel,
) (*ent.Model, []string, error) {
	target, err := svc.entFromContext(ctx).Model.Get(ctx, targetModelID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, xerrors.NotFoundError(fmt.Sprintf("model with id '%d'", targetModelID))
		}

		return nil, nil, fmt.Errorf("failed to query target model: %w", err)
	}

	settings := target.Settings
	if settings == nil {
		settings = &objects.ModelSettings{}
	}

	existing := settings.Associations

	added, warnings := buildImportedAssociations(target.ModelID, sources, existing, channels)
	if len(existing)+len(added) > MaxModelAssociations {
		return nil, nil, associationLimitError(target.ModelID, len(existing)+len(added))
	}

	if len(added) == 0 {
		// Every source is already associated; leave the Model untouched.
		return target, warnings, nil
	}

	updated := &objects.ModelSettings{
		DisableDeveloperSettingsInheritance: settings.DisableDeveloperSettingsInheritance,
		LoadBalancerStrategy:                settings.LoadBalancerStrategy,
		TraceStickyMode:                     settings.TraceStickyMode,
		Associations:                        append(append([]*objects.ModelAssociation{}, existing...), added...),
	}

	if err := validateModelSettings(updated); err != nil {
		return nil, nil, err
	}

	saved, err := svc.entFromContext(ctx).Model.UpdateOne(target).
		SetSettings(updated).
		Save(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to append model associations: %w", err)
	}

	return saved, warnings, nil
}

// buildImportedAssociations turns upstream sources into channel_model
// associations, skipping pairs the Model already carries.
//
// Each source gets its own increasing priority, so the imported rules form an
// ordered failover chain in the order the user picked them. That is the reason
// imports use channel_model at all: one rule per channel makes the sequence
// explicit, which the tag- and regex-based types cannot express because every
// channel a single rule matches shares one priority.
//
// Priorities are clamped to MaxModelAssociations, matching the ceiling the
// association dialog accepts, so appending onto already high priorities cannot
// produce a value the editor would reject.
func buildImportedAssociations(
	requestModelID string,
	sources []*ImportModelSource,
	existing []*objects.ModelAssociation,
	channels map[int]*Channel,
) ([]*objects.ModelAssociation, []string) {
	nextPriority := 0
	if len(existing) > 0 {
		nextPriority = lo.Max(lo.Map(existing, func(assoc *objects.ModelAssociation, _ int) int {
			return assoc.Priority
		})) + 1
	}

	seen := make(map[ChannelModelKey]bool, len(existing))

	for _, assoc := range existing {
		if assoc.Type == associationTypeChannelModel && assoc.ChannelModel != nil {
			seen[ChannelModelKey{
				ChannelID: assoc.ChannelModel.ChannelID,
				ModelID:   assoc.ChannelModel.ModelID,
			}] = true
		}
	}

	associations := make([]*objects.ModelAssociation, 0, len(sources))
	warnings := make([]string, 0)

	for _, source := range sources {
		key := ChannelModelKey{ChannelID: source.ChannelID, ModelID: source.UpstreamModelID}
		if seen[key] {
			warnings = append(warnings, fmt.Sprintf(
				"%s is already associated with model '%s' on channel %d, skipped",
				source.UpstreamModelID, requestModelID, source.ChannelID,
			))

			continue
		}

		seen[key] = true

		if warning := checkUpstreamModelServed(source, channels); warning != "" {
			warnings = append(warnings, warning)
		}

		associations = append(associations, &objects.ModelAssociation{
			Type:     associationTypeChannelModel,
			Priority: min(nextPriority+len(associations), MaxModelAssociations),
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: source.ChannelID,
				ModelID:   source.UpstreamModelID,
			},
		})
	}

	return associations, warnings
}

// checkUpstreamModelServed reports a warning when the channel does not resolve
// the given model ID. It never blocks the import: aliasing a request model onto
// an upstream name the channel does not list is a legitimate setup, so the
// mismatch can only be surfaced, not rejected.
//
// The oracle must be GetModelEntries, not SupportedModels: matchChannelModel
// looks the ID up in that map, so its keys are exactly the IDs a channel_model
// rule can match. They include the prefixed, auto-trimmed and mapped variants
// that SupportedModels does not carry, and exclude what HideOriginalModels and
// HideMappedModels remove.
func checkUpstreamModelServed(source *ImportModelSource, channels map[int]*Channel) string {
	ch, ok := channels[source.ChannelID]
	if !ok {
		return fmt.Sprintf("channel %d was not found, its rules may never match", source.ChannelID)
	}

	if _, served := ch.GetModelEntries()[source.UpstreamModelID]; served {
		return ""
	}

	return fmt.Sprintf(
		"channel '%s' does not serve model '%s'; requests may fail unless the provider accepts it",
		ch.Name, source.UpstreamModelID,
	)
}

func associationLimitError(modelID string, count int) error {
	return xerrors.ValidationError(fmt.Sprintf(
		"model '%s' would end up with %d association rules, the maximum is %d",
		modelID, count, MaxModelAssociations,
	))
}
