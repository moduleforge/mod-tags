import { useState, useEffect } from 'react';
import { ErrorBanner, useApiError } from '@moduleforge/core-gui';
import { TagChip } from './TagChip';
import type { Tag, TagsClientOptions } from './lib/api';
import { createTagsClient, ApiRequestError } from './lib/api';

export interface TagListProps {
  /** Subject entity UUID */
  subject: string;
  /**
   * Restrict to these purposes. undefined and [] are equivalent — both mean
   * "all purposes / free-form". This is enforced at the type level via
   * `purposes?: string[]` and guarded at runtime by checking `.length > 0`.
   */
  purposes?: string[];
  noPurpose?: boolean;
  client: ReturnType<typeof createTagsClient>;
  className?: string;
}

type LoadState = 'idle' | 'loading' | 'error' | 'ready';

/**
 * Wraps a caught value as an `ApiRequestError`. Every failure `client`
 * surfaces already comes through `@moduleforge/core-gui`'s `request()` as an
 * `ApiRequestError` (task 001); the fallback here is defensive only, in case
 * a caller-supplied `client` implementation (e.g. a test double) throws
 * something else.
 */
function toApiRequestError(err: unknown, fallbackMessage: string): ApiRequestError {
  return err instanceof ApiRequestError ? err : new ApiRequestError('internal_error', fallbackMessage, 0);
}

export function TagList({ subject, purposes, noPurpose, client, className }: TagListProps) {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loadState, setLoadState] = useState<LoadState>('idle');
  const [loadError, setLoadError] = useState<ApiRequestError | null>(null);

  // Stable key: purposes=undefined and purposes=[] both stringify to "[]"
  const purposesKey = JSON.stringify(purposes ?? []);

  // Load errors have no bindable inputs here, so every inline-classified
  // error routes to the banner; toast-worthy classes (network_error,
  // internal_error) are dispatched to the Toast provider by the hook itself.
  const { bannerError } = useApiError(loadError);

  useEffect(() => {
    let cancelled = false;
    setLoadState('loading');
    setLoadError(null);

    client
      .listBySubject(subject, purposes)
      .then((fetched) => {
        if (cancelled) return;
        setTags(fetched);
        setLoadState('ready');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(toApiRequestError(err, 'Failed to load tags.'));
        setLoadState('error');
      });

    return () => {
      cancelled = true;
    };
    // purposesKey is the stable serialization of purposes for the dep array
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subject, purposesKey]);

  if (loadState === 'loading' || loadState === 'idle') {
    return (
      <div className={`flex flex-wrap gap-1.5 ${className ?? ''}`} aria-busy="true">
        {/* Skeleton placeholders */}
        <span className="h-5 w-20 animate-pulse rounded-full bg-gray-200" />
        <span className="h-5 w-16 animate-pulse rounded-full bg-gray-200" />
      </div>
    );
  }

  if (loadState === 'error') {
    return <ErrorBanner error={bannerError} />;
  }

  // Empty state: render nothing — keeps the component unobtrusive
  if (tags.length === 0) {
    return null;
  }

  return (
    <div className={`flex flex-wrap gap-1.5 ${className ?? ''}`}>
      {tags.map((tag) => (
        <TagChip key={tag.uuid} tag={tag} noPurpose={noPurpose} />
      ))}
    </div>
  );
}

// Re-export for convenience of consumers who only import from TagList
export type { Tag, TagsClientOptions };
