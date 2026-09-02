import type { ComponentType } from 'react';
import * as Icons from '@lobehub/icons';
import { ProviderIcon } from '@lobehub/icons';
// Deep import: providerMappings is not re-exported from the package index, but
// the package has no "exports" map, so this resolves fine and carries types.
import { providerMappings } from '@lobehub/icons/es/features/providerConfig';
import { cn } from '@/lib/utils';
import type { Model } from '../data/schema';

type IconComponent = ComponentType<{ className?: string; size?: number }>;

/** Keywords @lobehub/icons can render a real brand avatar for (lowercased). */
const knownProviderKeywords = new Set(providerMappings.flatMap((m) => m.keywords.map((k) => k.toLowerCase())));

/**
 * Resolves the icon component recorded on the model (a @lobehub/icons export
 * name stored in the model's `icon` field), falling back to undefined so
 * callers can render the developer logo instead.
 */
export function resolveModelIcon(model: Pick<Model, 'icon'>): IconComponent | undefined {
  if (!model.icon) return undefined;
  const registry = Icons as unknown as Record<string, IconComponent>;
  return registry[model.icon];
}

/** Deterministic brand-ish hue for providers @lobehub/icons doesn't cover. */
function providerHue(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) | 0;
  }
  return Math.abs(hash) % 360;
}

/**
 * First-letter brand block for unknown providers (mockup: colored letter
 * block approximating the vendor logo). Static hue like the ReactFlow
 * segment bars — per-item distinct colors can't map to the token set.
 */
function ProviderLetterBlock({ name, size, className }: { name: string; size: number; className?: string }) {
  const letter = (name.trim()[0] ?? '?').toUpperCase();
  return (
    <span
      aria-hidden
      className={cn('grid shrink-0 place-items-center font-bold text-white', className)}
      style={{ width: size, height: size, fontSize: Math.max(8, Math.round(size / 2)), backgroundColor: `hsl(${providerHue(name)} 55% 45%)` }}
    >
      {letter}
    </span>
  );
}

function isKnownProvider(developer: string): boolean {
  return knownProviderKeywords.has(developer.toLowerCase());
}

/** 24px cell logo: the model's own icon, else the developer's brand avatar block. */
export function ModelLogo({ model, size = 24 }: { model: Model; size?: number }) {
  const IconComponent = resolveModelIcon(model);
  if (IconComponent) {
    return <IconComponent className='shrink-0' size={size} />;
  }
  if (!isKnownProvider(model.developer)) {
    return <ProviderLetterBlock name={model.developer} size={size} className='rounded-md' />;
  }
  return <ProviderIcon provider={model.developer} size={size} shape='square' type='avatar' className='rounded-md' />;
}

/** Compact 16px family logo used inside the model ID cell (no avatar block). */
export function ModelFamilyLogo({ model }: { model: Model }) {
  const IconComponent = resolveModelIcon(model);
  if (IconComponent) {
    return <IconComponent className='shrink-0' size={16} />;
  }
  if (!isKnownProvider(model.developer)) {
    return <ProviderLetterBlock name={model.developer} size={16} className='rounded-[5px]' />;
  }
  return <ProviderIcon provider={model.developer} size={16} type='color' className='shrink-0' />;
}

/** Developer group header logo (28px brand avatar). */
export function DeveloperLogo({ developer, size = 28 }: { developer: string; size?: number }) {
  if (!isKnownProvider(developer)) {
    return <ProviderLetterBlock name={developer} size={size} className='rounded-lg' />;
  }
  return <ProviderIcon provider={developer} size={size} shape='square' type='avatar' className='rounded-lg' />;
}
