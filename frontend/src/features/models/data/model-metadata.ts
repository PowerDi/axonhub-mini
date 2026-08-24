import { DEVELOPER_ICONS, DEVELOPER_IDS } from './constants';
import { type Provider, type ProviderModel, type ProvidersData, resolveVision } from './providers.schema';
import { type ModelCard, type ModelType, modelTypeSchema } from './schema';

/**
 * Normalizes a model ID down to a comparison key.
 *
 * Upstream IDs carry vendor prefixes and inconsistent separators for what is the
 * same model — `z-ai/glm-5.2`, `glm_5.2` and `GLM-5.2` all name one thing. The key
 * drops the prefix and every separator so those collapse onto each other, which is
 * what both the providers.json lookup and the "similar existing Model" hint need.
 */
export function normalizeModelKey(modelId: string): string {
  const withoutVendor = modelId.slice(modelId.lastIndexOf('/') + 1);
  return withoutVendor.toLowerCase().replace(/[-_.:\s]/g, '');
}

export type ModelSimilarity = 'exact' | 'similar' | 'none';

/**
 * Compares two model IDs the way a human skimming the list would: ignoring vendor
 * prefix and separators, then treating a shared beginning as related. It exists to
 * sort likely targets to the top of the dropdown, never to pick one automatically —
 * `glm-5.2` and `glm-5.2-think` are different models that must stay distinct.
 */
export function compareModelIds(a: string, b: string): ModelSimilarity {
  const keyA = normalizeModelKey(a);
  const keyB = normalizeModelKey(b);

  if (!keyA || !keyB) return 'none';
  if (keyA === keyB) return 'exact';
  if (keyA.startsWith(keyB) || keyB.startsWith(keyA)) return 'similar';

  return 'none';
}

interface DeveloperModel {
  developer: string;
  model: ProviderModel;
}

/**
 * Flattens providers.json into a lookup keyed by normalized model ID.
 *
 * Only entries whose provider key is a known developer are indexed: providers.json
 * also lists relays that re-serve other developers' models, and taking metadata
 * from those would attribute the model to the wrong developer. Earlier developers
 * win on collision so the index stays stable across calls.
 */
export function buildProviderModelIndex(data?: ProvidersData): Map<string, DeveloperModel> {
  const index = new Map<string, DeveloperModel>();
  if (!data) return index;

  for (const [developer, provider] of Object.entries(data.providers) as Array<[string, Provider]>) {
    if (!DEVELOPER_IDS.includes(developer)) continue;

    for (const model of provider.models || []) {
      const key = normalizeModelKey(model.id);
      if (key && !index.has(key)) {
        index.set(key, { developer, model });
      }
    }
  }

  return index;
}

function buildModelCard(model: ProviderModel): ModelCard {
  return {
    reasoning: {
      supported: model.reasoning?.supported || false,
      default: model.reasoning?.default || false,
    },
    toolCall: model.tool_call || false,
    temperature: model.temperature || false,
    modalities: {
      input: model.modalities?.input || [],
      output: model.modalities?.output || [],
    },
    vision: resolveVision(model),
    cost: {
      input: model.cost?.input || 0,
      output: model.cost?.output || 0,
      cacheRead: model.cost?.cache_read,
      cacheWrite: model.cost?.cache_write,
    },
    limit: {
      context: model.limit?.context || 0,
      output: model.limit?.output || 0,
    },
    knowledge: model.knowledge,
    releaseDate: model.release_date,
    lastUpdated: model.last_updated,
  };
}

// Stands in for the developer of a model providers.json does not know and whose
// ID carries no vendor prefix. It is deliberately conspicuous so the row reads as
// "needs review" rather than silently filing the model under a real developer.
const PLACEHOLDER_DEVELOPER = 'unknown';

const PLACEHOLDER_MODEL_CARD: ModelCard = {
  reasoning: { supported: false, default: false },
  toolCall: false,
  temperature: false,
  modalities: { input: [], output: [] },
  vision: false,
  cost: { input: 0, output: 0 },
  limit: { context: 0, output: 0 },
};

export interface DerivedModelMetadata {
  modelId: string;
  developer: string;
  type: ModelType;
  name: string;
  icon: string;
  group: string;
  modelCard: ModelCard;
  /** False when providers.json had no entry, so the fields below are placeholders. */
  matched: boolean;
}

/**
 * Derives the metadata a new Model needs from an upstream model ID.
 *
 * providers.json only tracks what upstream has published, so a freshly released
 * model legitimately has no entry. That is not an error: the import fills
 * placeholders (the bare ID as the name, the vendor prefix as the developer) and
 * flags `matched: false` so the UI can say the row needs review. Blocking here
 * would make the feature useless for exactly the models users import most.
 */
export function deriveModelMetadata(upstreamModelId: string, index: Map<string, DeveloperModel>): DerivedModelMetadata {
  // Strip the vendor prefix: `z-ai/glm-5.2` is registered as `glm-5.2` and the
  // prefix is a routing artifact of the channel, not part of the model identity.
  const modelId = upstreamModelId.slice(upstreamModelId.lastIndexOf('/') + 1);
  const match = index.get(normalizeModelKey(upstreamModelId));

  if (!match) {
    // The vendor prefix is the best developer guess available without a match.
    // Every field must stay non-empty: the Model schema accepts empty strings but
    // the table and icon lookups render them as blanks, so a placeholder that
    // shows what needs fixing beats one that shows nothing.
    const vendor = upstreamModelId.includes('/') ? upstreamModelId.slice(0, upstreamModelId.indexOf('/')) : '';
    const developer = vendor || PLACEHOLDER_DEVELOPER;

    return {
      modelId,
      developer,
      type: 'chat',
      name: modelId,
      icon: DEVELOPER_ICONS[developer] || developer,
      group: developer,
      modelCard: PLACEHOLDER_MODEL_CARD,
      matched: false,
    };
  }

  const { developer, model } = match;
  // providers.json spells types with hyphens (`image-generation`) while the schema
  // uses underscores; anything unrecognized falls back to chat rather than failing.
  const normalizedType = model.type?.replace(/-/g, '_');
  const type = normalizedType && modelTypeSchema.safeParse(normalizedType).success ? (normalizedType as ModelType) : 'chat';

  return {
    modelId,
    developer,
    type,
    name: model.display_name || model.name || modelId,
    icon: DEVELOPER_ICONS[developer] || developer,
    group: model.family || developer,
    modelCard: buildModelCard(model),
    matched: true,
  };
}
